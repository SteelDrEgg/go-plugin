package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"os"

	wasmpb "example.com/my-go-plugin-example/api/wasm/proto"

	grpcpb "example.com/my-go-plugin-example/api/grpc/proto"

	goplugin "github.com/SteelDrEgg/go-plugin"
)

const (
	pythonPackage = "dist/greeter_python.plg"
	wasmPackage   = "dist/greeter_wasm.plg"
)

var handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "GREETER_GRPC_PLUGIN",
	MagicCookieValue: "hello",
}

func main() {
	ctx := context.Background()

	mgr, err := goplugin.NewManager(goplugin.Config{
		TempDir: "tmp",
		GRPC: &goplugin.GRPCConfig{
			HandshakeConfig: handshake,
			AllowedProtocols: []goplugin.Protocol{
				goplugin.ProtocolGRPC,
			},
			SyncStderr: os.Stderr,
			Loader: func(_ context.Context, c *grpc.ClientConn) (any, error) {
				return grpcpb.NewGreeterClient(c), nil
			},
		},
		WASM: &goplugin.WASMConfig{
			Loader: loadWasmGreeter,
		},
	})
	if err != nil {
		panic(fmt.Errorf("new manager: %w", err))
	}

	if err := callPythonPlugin(ctx, mgr); err != nil {
		panic(err)
	}
	if err := callWasmPlugin(ctx, mgr); err != nil {
		panic(err)
	}
}

func callPythonPlugin(ctx context.Context, mgr *goplugin.Manager) error {
	handle, err := mgr.Load(pythonPackage)
	if err != nil {
		return fmt.Errorf("load python plugin: %w", err)
	}
	defer mgr.Unload(handle)

	fmt.Println("===== Current Plugin:", handle.Info().Metadata["DisplayName"], "=====")

	client, ok := handle.Client().(grpcpb.GreeterClient)
	if !ok {
		return fmt.Errorf("unexpected python plugin client type %T", handle.Client())
	}

	resp, err := client.SayHello(ctx, &grpcpb.GreetRequest{Name: "python grpc"})
	if err != nil {
		return fmt.Errorf("call python plugin: %w", err)
	}
	fmt.Printf("[python grpc] %s\n", resp.GetMessage())
	return nil
}

func callWasmPlugin(ctx context.Context, mgr *goplugin.Manager) error {
	handle, err := mgr.Load(wasmPackage)
	if err != nil {
		return fmt.Errorf("load wasm plugin: %w", err)
	}
	defer mgr.Unload(handle)

	fmt.Println("===== Current Plugin:", handle.Info().Metadata["DisplayName"], "=====")

	client, ok := handle.Client().(wasmpb.Greeter)
	if !ok {
		return fmt.Errorf("unexpected wasm plugin client type %T", handle.Client())
	}

	resp, err := client.SayHello(ctx, &wasmpb.GreetRequest{Name: "golang wasm"})
	if err != nil {
		return fmt.Errorf("call wasm plugin: %w", err)
	}
	fmt.Printf("[golang wasm] %s\n", resp.GetMessage())
	return nil
}

func loadWasmGreeter(ctx context.Context, modulePath string, _ goplugin.Info) (any, func(context.Context) error, error) {
	loader, err := wasmpb.NewGreeterPlugin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("new wasm loader: %w", err)
	}

	client, err := loader.Load(ctx, modulePath, hostFunctions{})
	if err != nil {
		return nil, nil, fmt.Errorf("load wasm binary: %w", err)
	}

	return client, func(ctx context.Context) error { return client.Close(ctx) }, nil
}

type hostFunctions struct{}

func (hostFunctions) Prefix(_ context.Context, req *wasmpb.PrefixRequest) (*wasmpb.PrefixReply, error) {
	return &wasmpb.PrefixReply{
		Text: "[host] " + req.GetText(),
	}, nil
}
