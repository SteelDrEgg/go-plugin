# Go Plugin System

`go-plugin` is a go plugin system that combined two most popular golang plugin systems: 
[hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) and [knqyf263/go-plugin](https://github.com/knqyf263/go-plugin), providing consistent
between gRPC and Wasm plugin experience.

## Architecture

### Plugin
Plugin is packs into a single `plg` file, essentially a `zip` file.

Say there is a greeter plugin, the structure is as shown in below
```text
greeter.plg
├── info.yaml
└── Content/
```

Content is a directory containing all resources used by the plugin, including executable, wasm, and static resources.

`info.yaml` defines the plugin. It looks like the following.
```yaml
# Required
Name: com.example/greeter
Version: 1.0.0
Type: grpc             # enum: grpc | wasm
ContractVersion: 1     # host check this for compatibility
Command: $PLUGIN_ROOT/greeter run
# Optional custom metadata
DisplayName: Greeter
Category: demo
```

In the case of wasm, field `Command` will be the location of wasm file

You can read an `info.yaml` file directly with `goplugin.ReadInfo`.
Required fields are mapped onto `Info`; any other fields are stored in `Info.Metadata`.

`plg` files are extracted to a temporary location before loading. Unpacked plugin
directories can also be loaded directly or copied to a temporary location first.

`$PLUGIN_ROOT` will be the location of `Content`

### Host
Plugins ane host are communicated using `protobuf3` protocol.

SDKs, or `pb` files will be generated from `proto` files.
They define interfaces and data structures used to communicate.

## Installation

This module uses [knqyf263/go-plugin](https://github.com/knqyf263/go-plugin) as backend for Wasm.
To develop Wasm plugin, you need a  [compiler](https://github.com/knqyf263/go-plugin/releases/latest).

To develop a plugin, `protobuf` is required, see [documentation](https://protobuf.dev/installation/) for instructions.

And install go module by
```shell
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

## Usage

For open box example, see [basic example](tree/main/examples/basic).

### Generate interface

If you're not provided `pb` files (sdk), you need to generate it from `proto` file, where interfaces are defined.

After that, generate SDK
```shell
protoc \
-I. \
--go_out=. --go_opt=paths=source_relative \
--go-grpc_out=. --go-grpc_opt=paths=source_relative \
<my-plugin>.proto
```

Then import into host
```go
import (
    pb "example.com/my-plugin/proto
)
```

### Initialize manager

Handshake defines some information that will be checked prior to establishing a gRPC connection.
```go
goplugin.HandshakeConfig{
    ProtocolVersion:  1,
    MagicCookieKey:   "GRPC_PLUGIN",
    MagicCookieValue: "hello",
}
```

Prepare for configs. Here binds services defined in `proto`
```go
goplugin.GRPCConfig{
    HandshakeConfig: handshake,
    // Set true to prevent the plugin subprocess from inheriting the host environment.
    SkipHostEnv: true,
    Loader: func(_ context.Context, c *grpc.ClientConn) (any, error) {
        return pb.NewPluginClient(c), nil
    },
},
```

Load the config
```go
mgr, err := goplugin.NewManager(goplugin.Config{
    // Optional: empty uses the system temporary directory.
    TempDir: "/tmp",
    GRPC: GRPCConfig,
    WASM: nil,
})
```

### Use plugin
Load the plugin
```go
handle, _ := mgr.Load("my-plugin.plg")
defer mgr.Unload(handle)
// pb.<PluginSDK> is a placeholder, definitions at .proto
client, _ := handle.Client().(pb.<PluginSDK>)
```

To load an unpacked directory, keep the same `info.yaml` and `Content/` layout:

```go
// Use the original directory. Unload never removes the source directory.
handle, err := mgr.LoadDir("./my-plugin", false)

// Or copy the directory to a private temporary directory before loading.
handle, err = mgr.LoadDir("./my-plugin", true)
```

The copy preserves file permissions, including executable bits. Copying supports
regular files and directories; symbolic links and special files are rejected.
`$PLUGIN_ROOT` and `handle.RootPath()` refer to the loaded `Content/` directory.
Copied resources are independent of later source changes; loading in place uses
the original resources. Always call `mgr.Unload(handle)` or `handle.Close(ctx)`
after a successful load. Temporary copies are removed on load failure or after
successful plugin cleanup during unload. If backend cleanup fails during load
rollback, the temporary directory is retained and its path is included in the
returned error. Cleanup errors are reported alongside the original load error.

### Lifecycle and cancellation

The application owns each returned `Handle` and decides when to unload it. Before
closing a handle, stop new plugin calls and resource reads, and wait for existing
operations to finish. `Client()` does not track calls or prevent use after close.

`LoadContext(ctx, path)`, `LoadDirContext(ctx, dir, copyToTemp)`, and
`UnloadContext(ctx, handle)` accept cancellation and deadlines. Existing `Load`,
`LoadDir`, and `Unload` use a background context. A startup context controls file
preparation and backend initialization; cancelling it after a successful load does
not close the plugin. Cancellation during file preparation is checked between
entries and reads; an already-blocked filesystem operation cannot be interrupted.

```go
loadCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
handle, err := mgr.LoadContext(loadCtx, "my-plugin.plg")
cancel()
if err != nil {
    return err
}
// Use the plugin, then stop and drain all calls before closing it.
closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
defer closeCancel()
if err := mgr.UnloadContext(closeCtx, handle); err != nil {
    // Keep the handle and retry Close with a fresh context.
    return err
}
```

Closing is serialized per handle. A failed close can be retried, and completed
steps are not repeated: backend cleanup succeeds before temporary files are
removed. A gRPC close deadline forces the default subprocess to exit while backend
cleanup finishes in the background; retry close to await completion and remove the
temporary directory. Once close succeeds, subsequent close calls return nil.

Loaders must honor the startup context and use separate contexts for later plugin
calls. A WASM Loader that allocates resources before returning an error must either
release them itself or return a cleanup function with the error. The manager runs
that function during rollback, using a context without startup cancellation, and
reports any cleanup error. Rollback can extend beyond the startup deadline.
Cleanup callbacks must honor their context and tolerate retries after failure.

Loads can run concurrently. Configuration must remain unchanged after manager
creation, and Loader and configuration override callbacks must support concurrent
invocation. gRPC overrides replacing the command, runner, or plugin presets must
provide equivalent startup cancellation and shutdown behavior themselves.

*WASM Client is not thread-safe, add a lock*

Call plugin methods
```go
resp, _ := client.Hello(ctx, &pb.<Params>{To: "World"})
fmt.Printf("Hello %s", resp.<GetMessage>())
```

### Read plugin resources

Each loaded plugin has a `Content` root directory. `Handle` can map resource addresses
to this root, so callers can expose a simple read API to plugins.

```go
// Both forms are supported:
// - "/greet.txt"
// - "my-plugin.plg/Content/greet.txt"
data, err := handle.ReadFile("/greet.txt")
if err != nil {
    // handle error
}
fmt.Printf("resource bytes: %d\n", len(data))
```
