package goplugin

import (
	"context"
	"fmt"
	"sort"

	hcplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

const defaultGRPCPresetPluginName = "default_grpc"

type grpcPresetPlugin struct {
	hcplugin.NetRPCUnsupportedPlugin
	loader           func(context.Context, *grpc.ClientConn) (any, error)
	loaderWithBroker func(context.Context, *GRPCBroker, *grpc.ClientConn) (any, error)
}

func (p *grpcPresetPlugin) GRPCServer(*hcplugin.GRPCBroker, *grpc.Server) error {
	return fmt.Errorf("host only plugin")
}

func (p *grpcPresetPlugin) GRPCClient(ctx context.Context, broker *hcplugin.GRPCBroker, conn *grpc.ClientConn) (any, error) {
	if p.loaderWithBroker != nil {
		return p.loaderWithBroker(ctx, wrapGRPCBroker(broker), conn)
	}
	if p.loader == nil {
		return conn, nil
	}
	return p.loader(ctx, conn)
}

func defaultGRPCPreset(cfg *GRPCConfig) map[string]hcplugin.Plugin {
	return map[string]hcplugin.Plugin{
		defaultGRPCPresetPluginName: &grpcPresetPlugin{
			loader:           cfg.Loader,
			loaderWithBroker: cfg.LoaderWithBroker,
		},
	}
}

func resolveDispenseName(plugins map[string]hcplugin.Plugin) string {
	if len(plugins) == 0 {
		return ""
	}
	if _, ok := plugins[defaultGRPCPresetPluginName]; ok {
		return defaultGRPCPresetPluginName
	}
	keys := make([]string, 0, len(plugins))
	for k := range plugins {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys[0]
}
