package main

import (
	"context"
	"fmt"
	"io"

	grpcpb "example.com/my-go-plugin-example/api/grpc/proto"
	hcplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const pluginName = "default_grpc"

var handshake = hcplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "GREETER_GRPC_PLUGIN",
	MagicCookieValue: "hello",
}

func main() {
	hcplugin.Serve(&hcplugin.ServeConfig{
		HandshakeConfig: handshake,
		Plugins: map[string]hcplugin.Plugin{
			pluginName: &greeterPlugin{},
		},
		GRPCServer: hcplugin.DefaultGRPCServer,
	})
}

type greeterPlugin struct {
	hcplugin.NetRPCUnsupportedPlugin
}

func (p *greeterPlugin) GRPCServer(broker *hcplugin.GRPCBroker, s *grpc.Server) error {
	grpcpb.RegisterGreeterCallbackServer(s, &greeterServer{broker: broker})
	return nil
}

func (p *greeterPlugin) GRPCClient(context.Context, *hcplugin.GRPCBroker, *grpc.ClientConn) (any, error) {
	return nil, fmt.Errorf("plugin process does not use GRPCClient")
}

type greeterServer struct {
	grpcpb.UnimplementedGreeterCallbackServer
	broker *hcplugin.GRPCBroker
}

func (s *greeterServer) SayHello(_ context.Context, req *grpcpb.GreetRequest) (*grpcpb.GreetReply, error) {
	return &grpcpb.GreetReply{
		Message: "hello from go grpc plugin: " + req.GetName(),
	}, nil
}

func (s *greeterServer) Chat(stream grpcpb.GreeterCallback_ChatServer) error {
	if err := stream.Send(&grpcpb.ChatMessage{
		From: "plugin",
		Text: "stream callback ready",
	}); err != nil {
		return err
	}

	for {
		msg, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if err := stream.Send(&grpcpb.ChatMessage{
			From: "plugin",
			Text: "echo: " + msg.GetText(),
		}); err != nil {
			return err
		}
	}
}

func (s *greeterServer) NotifyHost(ctx context.Context, req *grpcpb.NotifyHostRequest) (*grpcpb.NotifyHostReply, error) {
	if s.broker == nil {
		return nil, fmt.Errorf("grpc broker is not available")
	}
	conn, err := s.broker.Dial(req.GetBrokerId())
	if err != nil {
		return nil, fmt.Errorf("dial host callback broker id %d: %w", req.GetBrokerId(), err)
	}
	defer conn.Close()

	callback := grpcpb.NewHostCallbackClient(conn)
	resp, err := callback.OnEvent(ctx, &grpcpb.CallbackEventRequest{
		Text: "callback from plugin via broker: " + req.GetMessage(),
	})
	if err != nil {
		return nil, fmt.Errorf("call host callback: %w", err)
	}

	return &grpcpb.NotifyHostReply{
		Result: "host callback ack: " + resp.GetAck(),
	}, nil
}

func (s *greeterServer) NotifyHostStd(ctx context.Context, req *grpcpb.NotifyHostStdRequest) (*grpcpb.NotifyHostStdReply, error) {
	conn, err := grpc.DialContext(
		ctx,
		req.GetHostAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial host callback server %q: %w", req.GetHostAddr(), err)
	}
	defer conn.Close()

	callback := grpcpb.NewHostCallbackClient(conn)
	resp, err := callback.OnEvent(ctx, &grpcpb.CallbackEventRequest{
		Text:  "callback from plugin via standard grpc: " + req.GetMessage(),
		Token: req.GetToken(),
	})
	if err != nil {
		return nil, fmt.Errorf("call host callback over standard grpc: %w", err)
	}

	return &grpcpb.NotifyHostStdReply{
		Result: "host callback ack: " + resp.GetAck(),
	}, nil
}
