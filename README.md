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

`plg` file will be extracted to a temporary location everytime before loading the plugin.

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
    Loader: func(_ context.Context, c *grpc.ClientConn) (any, error) {
        return pb.NewPluginClient(c), nil
    },
},
```

Load the config
```go
mgr, err := goplugin.NewManager(goplugin.Config{
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
