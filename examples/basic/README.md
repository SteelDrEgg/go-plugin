# Basic Example

This is an example shows how to use `SteelDrEgg/go-plugin`

## Files

```text
basic
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
===== Current Plugin: PythonGreeter =====
[python grpc] hello from python grpc plugin: python grpc

===== Current Plugin: WasmGreeter =====
[golang wasm] hello from wasm plugin: [host] golang wasm | file: hello from wasm resource
```