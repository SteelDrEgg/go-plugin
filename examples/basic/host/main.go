package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	wasmpb "example.com/my-go-plugin-example/api/wasm/proto"

	grpcpb "example.com/my-go-plugin-example/api/grpc/proto"

	goplugin "github.com/SteelDrEgg/go-plugin"
	"google.golang.org/grpc"
)

const (
	goGRPCPackage = "dist/greeter_go_grpc.plg"
	pythonPackage = "dist/greeter_python.plg"
	wasmPackage   = "dist/greeter_wasm.plg"
)

var handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "GREETER_GRPC_PLUGIN",
	MagicCookieValue: "hello",
}

var currentWasmHandle = newWasmHandleStore()

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
			LoaderWithBroker: func(_ context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
				return &grpcClientWithBroker{
					greeter: grpcpb.NewGreeterCallbackClient(c),
					broker:  broker,
				}, nil
			},
		},
		WASM: &goplugin.WASMConfig{
			ClientConfigOverride: func(cfg *goplugin.WASMClientConfig) {
				// If don't explicitly specify, wasm could not access real clock
				cfg.ModuleConfig = cfg.ModuleConfig.
					WithSysWalltime().
					WithSysNanotime()
			},
			Loader: loadWasmGreeter,
		},
	})
	if err != nil {
		panic(fmt.Errorf("new manager: %w", err))
	}

	if err := callGRPCPlugin(ctx, mgr); err != nil {
		panic(err)
	}
	if err := callPythonPlugin(ctx, mgr); err != nil {
		panic(err)
	}
	if err := callWasmPlugin(ctx, mgr); err != nil {
		panic(err)
	}
}

func callGRPCPlugin(ctx context.Context, mgr *goplugin.Manager) error {
	return callGRPCCallbackPlugin(ctx, mgr, goGRPCPackage, true, "go grpc")
}

func callPythonPlugin(ctx context.Context, mgr *goplugin.Manager) error {
	return callGRPCCallbackPlugin(ctx, mgr, pythonPackage, false, "python grpc")
}

func callGRPCCallbackPlugin(ctx context.Context, mgr *goplugin.Manager, pluginPath string, supportsBroker bool, label string) error {
	handle, err := mgr.Load(pluginPath)
	if err != nil {
		return fmt.Errorf("load %s plugin: %w", label, err)
	}
	defer mgr.Unload(handle)

	fmt.Println("===== Current Plugin:", handle.Info().Metadata["DisplayName"], "=====")

	client, ok := handle.Client().(*grpcClientWithBroker)
	if !ok {
		return fmt.Errorf("unexpected grpc plugin client type %T", handle.Client())
	}

	resp, err := client.greeter.SayHello(ctx, &grpcpb.GreetRequest{Name: label})
	if err != nil {
		return fmt.Errorf("call %s plugin say hello: %w", label, err)
	}
	fmt.Printf("[%s unary] %s\n", label, resp.GetMessage())

	if err := callGRPCBidirectionalStream(ctx, label, client.greeter); err != nil {
		return err
	}
	if err := callGRPCStandardCallback(ctx, label, client.greeter); err != nil {
		return err
	}
	if supportsBroker {
		if err := callGRPCBrokerCallback(ctx, label, client.broker, client.greeter); err != nil {
			return err
		}
	}

	return nil
}

func callGRPCBidirectionalStream(ctx context.Context, label string, client grpcpb.GreeterCallbackClient) error {
	stream, err := client.Chat(ctx)
	if err != nil {
		return fmt.Errorf("open %s stream: %w", label, err)
	}

	first, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receive first %s stream callback: %w", label, err)
	}
	fmt.Printf("[%s bidi] %s: %s\n", label, first.GetFrom(), first.GetText())

	if err := stream.Send(&grpcpb.ChatMessage{
		From: "host",
		Text: "hello stream",
	}); err != nil {
		return fmt.Errorf("send stream message: %w", err)
	}

	reply, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receive %s stream reply: %w", label, err)
	}
	fmt.Printf("[%s bidi] %s: %s\n", label, reply.GetFrom(), reply.GetText())

	if err := stream.CloseSend(); err != nil {
		return fmt.Errorf("close stream: %w", err)
	}
	return nil
}

func callGRPCStandardCallback(ctx context.Context, label string, client grpcpb.GreeterCallbackClient) error {
	token := "std-callback-token"
	addr, stop, err := startHostCallbackServer(hostCallbackServer{
		logTag:        label + " standard callback",
		expectedToken: token,
	})
	if err != nil {
		return fmt.Errorf("start %s callback server: %w", label, err)
	}
	defer stop()

	resp, err := client.NotifyHostStd(ctx, &grpcpb.NotifyHostStdRequest{
		HostAddr: addr,
		Token:    token,
		Message:  "from host standard notify request",
	})
	if err != nil {
		return fmt.Errorf("notify %s plugin over standard grpc callback: %w", label, err)
	}
	fmt.Printf("[%s standard] %s\n", label, resp.GetResult())
	return nil
}

func callGRPCBrokerCallback(ctx context.Context, label string, broker *goplugin.GRPCBroker, client grpcpb.GreeterCallbackClient) error {
	if broker == nil {
		return fmt.Errorf("grpc broker is not available on host")
	}

	brokerID := broker.NextID()
	go broker.AcceptAndServe(brokerID, func(s *grpc.Server) {
		grpcpb.RegisterHostCallbackServer(s, hostCallbackServer{
			logTag: label + " broker callback",
		})
	})

	resp, err := client.NotifyHost(ctx, &grpcpb.NotifyHostRequest{
		BrokerId: brokerID,
		Message:  "from host notify request",
	})
	if err != nil {
		return fmt.Errorf("notify %s plugin to callback host by broker: %w", label, err)
	}
	fmt.Printf("[%s broker] %s\n", label, resp.GetResult())
	return nil
}

func startHostCallbackServer(handler hostCallbackServer) (addr string, stop func(), err error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	server := grpc.NewServer()
	grpcpb.RegisterHostCallbackServer(server, handler)

	go func() {
		_ = server.Serve(listener)
	}()

	return listener.Addr().String(), func() {
		server.GracefulStop()
		_ = listener.Close()
	}, nil
}

func callWasmPlugin(ctx context.Context, mgr *goplugin.Manager) error {
	handle, err := mgr.Load(wasmPackage)
	if err != nil {
		return fmt.Errorf("load wasm plugin: %w", err)
	}
	defer mgr.Unload(handle)
	currentWasmHandle.Set(handle)
	defer currentWasmHandle.Set(nil)

	fmt.Println("===== Current Plugin:", handle.Info().Metadata["DisplayName"], "=====")
	// IMPORTANT!
	// THIS IS NOT TREAD SAFE, ADD A LOCK IN PRODUCTION!
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

func loadWasmGreeter(ctx context.Context, modulePath string, _ goplugin.Info, runtimeCfg *goplugin.WASMClientConfig) (any, func(context.Context) error, error) {
	loader, err := wasmpb.NewGreeterPlugin(
		ctx,
		wasmpb.WazeroRuntime(runtimeCfg.NewRuntime),
		wasmpb.WazeroModuleConfig(runtimeCfg.ModuleConfig),
	)
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

func (hostFunctions) ReadFile(_ context.Context, req *wasmpb.ReadFileRequest) (*wasmpb.ReadFileReply, error) {
	handle := currentWasmHandle.Get()
	if handle == nil {
		return nil, fmt.Errorf("wasm plugin handle is not ready")
	}
	data, err := handle.ReadFile(req.GetPath())
	if err != nil {
		return nil, err
	}
	return &wasmpb.ReadFileReply{Data: data}, nil
}

type hostCallbackServer struct {
	grpcpb.UnimplementedHostCallbackServer
	logTag        string
	expectedToken string
}

func (s hostCallbackServer) OnEvent(_ context.Context, req *grpcpb.CallbackEventRequest) (*grpcpb.CallbackEventReply, error) {
	if s.expectedToken != "" && req.GetToken() != s.expectedToken {
		return nil, fmt.Errorf("invalid callback token")
	}
	fmt.Printf("[%s] %s\n", s.logTag, req.GetText())
	return &grpcpb.CallbackEventReply{
		Ack: "host received callback",
	}, nil
}

type grpcClientWithBroker struct {
	greeter grpcpb.GreeterCallbackClient
	broker  *goplugin.GRPCBroker
}

type wasmHandleStore struct {
	mu     sync.RWMutex
	handle *goplugin.Handle
}

func newWasmHandleStore() *wasmHandleStore {
	return &wasmHandleStore{}
}

func (s *wasmHandleStore) Set(h *goplugin.Handle) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handle = h
}

func (s *wasmHandleStore) Get() *goplugin.Handle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.handle
}
