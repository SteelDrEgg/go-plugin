package goplugin

import (
	hcplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

// GRPCBroker wraps hashicorp go-plugin broker for host-side usage.
type GRPCBroker struct {
	raw *hcplugin.GRPCBroker
}

func wrapGRPCBroker(raw *hcplugin.GRPCBroker) *GRPCBroker {
	if raw == nil {
		return nil
	}
	return &GRPCBroker{raw: raw}
}

// NextID returns a unique broker stream ID.
func (b *GRPCBroker) NextID() uint32 {
	if b == nil || b.raw == nil {
		return 0
	}
	return b.raw.NextId()
}

// AcceptAndServe serves a gRPC server on broker stream id.
func (b *GRPCBroker) AcceptAndServe(id uint32, register func(*grpc.Server)) {
	if b == nil || b.raw == nil || register == nil {
		return
	}
	b.raw.AcceptAndServe(id, func(opts []grpc.ServerOption) *grpc.Server {
		s := grpc.NewServer(opts...)
		register(s)
		return s
	})
}
