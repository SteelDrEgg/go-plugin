PROTOC_GEN_GO := $(shell go env GOPATH)/bin/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(shell go env GOPATH)/bin/protoc-gen-go-grpc
PROTOC_GEN_GO_PLUGIN ?= $(abspath ../../go-plugin_0.9.0_darwin-amd64/protoc-gen-go-plugin)
PYTHON_VENV ?= ./python_plugin/venv
PYTHON_GRPC_TOOLS ?= $(PYTHON_VENV)/bin/python
PYTHON_RUNTIME := $(abspath $(PYTHON_GRPC_TOOLS))

.PHONY: deps tidy proto-go proto-py proto-wasm proto wasm package-python package-wasm package all run clean

deps:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	python3 -m venv $(PYTHON_VENV)
	$(PYTHON_GRPC_TOOLS) -m pip install -r ./python_plugin/requirements.txt

tidy:
	go mod tidy

proto-go:
	mkdir -p ./api/grpc
	PATH="$(shell go env GOPATH)/bin:$(PATH)" protoc \
		-I. \
		--go_out=./api/grpc --go_opt=paths=source_relative \
		--go-grpc_out=./api/grpc --go-grpc_opt=paths=source_relative \
		./proto/greeter.proto

proto-py:
	$(PYTHON_GRPC_TOOLS) -m grpc_tools.protoc \
		-I./proto \
		--python_out=./python_plugin/proto \
		--grpc_python_out=./python_plugin/proto \
		./proto/greeter.proto

proto-wasm:
	mkdir -p ./api/wasm
	protoc --plugin=protoc-gen-go-plugin=$(PROTOC_GEN_GO_PLUGIN) \
		-I. \
		--go-plugin_out=./api/wasm --go-plugin_opt=paths=source_relative \
		./proto/greeter.proto

proto: proto-go proto-py proto-wasm

wasm:
	mkdir -p dist
	GOOS=wasip1 GOARCH=wasm go build -o ./dist/greeter.wasm -buildmode=c-shared ./wasm_plugin

go-grpc:
	mkdir -p dist
	go build -o ./dist/greeter_go_grpc ./go_grpc_plugin

package-python:
	mkdir -p dist
	rm -rf ./dist/python_pkg && mkdir -p ./dist/python_pkg/Content/proto
	cp ./python_plugin/plugin.py ./dist/python_pkg/Content/plugin.py
	cp ./python_plugin/proto/*.py ./dist/python_pkg/Content/proto/
	printf 'Name: com.example/greeter-python\nVersion: 1.0.0\nType: grpc\nContractVersion: 1\nCommand: $(PYTHON_RUNTIME) $$PLUGIN_ROOT/plugin.py\nDisplayName: PythonGreeter\nCategory: basic-example\n' > ./dist/python_pkg/info.yaml
	cd ./dist/python_pkg && zip -qr ../greeter_python.plg .
	rm -rf ./dist/python_pkg

package-wasm: wasm
	mkdir -p dist
	rm -rf ./dist/wasm_pkg && mkdir -p ./dist/wasm_pkg/Content
	cp ./dist/greeter.wasm ./dist/wasm_pkg/Content/greeter.wasm
	cp ./wasm_plugin/greet.txt ./dist/wasm_pkg/Content/greet.txt
	printf 'Name: com.example/greeter-wasm\nVersion: 1.0.0\nType: wasm\nContractVersion: 1\nCommand: $$PLUGIN_ROOT/greeter.wasm\nDisplayName: WasmGreeter\nCategory: basic-example\n' > ./dist/wasm_pkg/info.yaml
	cd ./dist/wasm_pkg && zip -qr ../greeter_wasm.plg .
	rm -rf ./dist/wasm_pkg

package-go-grpc: go-grpc
	mkdir -p dist
	rm -rf ./dist/go_grpc_pkg && mkdir -p ./dist/go_grpc_pkg/Content
	cp ./dist/greeter_go_grpc ./dist/go_grpc_pkg/Content/greeter-go-grpc
	printf 'Name: com.example/greeter-go-grpc\nVersion: 1.0.0\nType: grpc\nContractVersion: 1\nCommand: $$PLUGIN_ROOT/greeter-go-grpc\nDisplayName: GoGRPCGreeter\nCategory: basic-example\n' > ./dist/go_grpc_pkg/info.yaml
	cd ./dist/go_grpc_pkg && zip -qr ../greeter_go_grpc.plg .
	rm -rf ./dist/go_grpc_pkg

package: package-go-grpc package-python package-wasm

all: deps proto tidy package

run:
	go run ./host

clean:
	rm -rf ./dist/greeter.wasm ./dist/greeter_go_grpc ./dist/greeter_go_grpc.plg ./dist/greeter_python.plg ./dist/greeter_wasm.plg ./tmp/* ./api/wasm/*/*.pb.go ./api/grpc/*/*.pb.go ./python_plugin/proto/*pb2*.py

purge: clean
	rm -rf ./python_plugin/venv