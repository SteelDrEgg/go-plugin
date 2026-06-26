# Basic Example

This is an example shows how to use `SteelDrEgg/go-plugin`

## Files

```text
basic
├── go_grpc_plugin
│   └── main.go
├── host
│   └── main.go
├── api
│   ├── grpc
│   │   └── proto
│   └── wasm
│       └── proto
├── proto
│   └── greeter.proto
├── wasm_plugin
│ └── main.go
├── python_plugin
│   ├── proto
│   ├── plugin.py
│   └── requirements.txt
├── tmp
├── dist
├── go.mod
├── Makefile
└── README.md
```

`api` holds all generated SDKs

`proto` defines SDK

## Run
Get latest wasm compiler at [knqyf263/go-plugin](https://github.com/knqyf263/go-plugin/releases/latest)

Export its location to `PROTOC_GEN_GO_PLUGIN` or set it in Makefile

If the structure of this project have changed, modify `go.mod` and set
 `github.com/SteelDrEgg/go-plugin` to its actual location or download from the internet.

Build everything
```shell
make all
```

Run everything
```shell
make run
```

It should output something like the following
```text
go run ./host
===== Current Plugin: GoGRPCGreeter =====
[go grpc unary] hello from go grpc plugin: go grpc
[go grpc bidi] plugin: stream callback ready
[go grpc bidi] plugin: echo: hello stream
[go grpc standard callback] callback from plugin via standard grpc: from host standard notify request
[go grpc standard] host callback ack: host received callback
[go grpc broker callback] callback from plugin via broker: from host notify request
[go grpc broker] host callback ack: host received callback

===== Current Plugin: PythonGreeter =====
[python grpc unary] hello from python grpc plugin: python grpc
[python grpc bidi] plugin: stream callback ready (python)
[python grpc bidi] plugin: echo: hello stream
[python grpc standard callback] callback from python plugin via standard grpc: from host standard notify request
[python grpc standard] host callback ack: host received callback

===== Current Plugin: WasmGreeter =====
[golang wasm] hello from wasm plugin: [host] golang wasm | file: hello from wasm resource
```

## Notes
For gRPC callback demos:
- Method 1 (bidirectional stream): host calls `GreeterCallback.Chat`, plugin sends stream messages back.
- Method 2 (standard callback): host starts a temporary gRPC `HostCallback` server on localhost,
  plugin dials that address and calls `HostCallback.OnEvent`.
- Method 3 (broker reverse channel, go plugin only): host starts `HostCallback` server with `GRPCBroker`,
  then plugin dials back via broker and calls `HostCallback.OnEvent`.

In this example, wasm plugin calls host function `ReadFile("/greet.txt")`.
Host resolves the path with `handle.ReadFile(...)`, so plugin resources are read
relative to that plugin's own `Content` root.