//go:build wasip1

package main

import (
	"context"
	"fmt"

	wasmgreeter "example.com/my-go-plugin-example/api/wasm/proto"
)

func main() {}

func init() {
	wasmgreeter.RegisterGreeter(wasmGreeter{})
}

type wasmGreeter struct{}

func (wasmGreeter) SayHello(ctx context.Context, req *wasmgreeter.GreetRequest) (*wasmgreeter.GreetReply, error) {
	hostFns := wasmgreeter.NewHostFunctions()
	prefixed, err := hostFns.Prefix(ctx, &wasmgreeter.PrefixRequest{Text: req.GetName()})
	if err != nil {
		return nil, err
	}

	fileReply, err := hostFns.ReadFile(ctx, &wasmgreeter.ReadFileRequest{Path: "/greet.txt"})
	if err != nil {
		return nil, err
	}

	return &wasmgreeter.GreetReply{
		Message: fmt.Sprintf(
			"hello from wasm plugin: %s | file: %s",
			prefixed.GetText(),
			string(fileReply.GetData()),
		),
	}, nil
}
