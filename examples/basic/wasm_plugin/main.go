//go:build wasip1

package main

import (
	"context"

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
	return &wasmgreeter.GreetReply{
		Message: "hello from wasm plugin: " + prefixed.GetText(),
	}, nil
}
