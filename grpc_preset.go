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
	loadCtx          context.Context
	observeExit      func(context.Context)
}

func (p *grpcPresetPlugin) GRPCServer(*hcplugin.GRPCBroker, *grpc.Server) error {
	return fmt.Errorf("host only plugin")
}

func (p *grpcPresetPlugin) GRPCClient(ctx context.Context, broker *hcplugin.GRPCBroker, conn *grpc.ClientConn) (any, error) {
	if p.observeExit != nil {
		p.observeExit(ctx)
	}
	// The loader's startup context carries caller values and deadlines, and
	// also stops if the backend exits. It must not be retained after startup.
	loaderCtx, cancel := context.WithCancel(p.loadCtx)
	defer cancel()
	stopLifetime := context.AfterFunc(ctx, cancel)
	defer stopLifetime()
	if err := p.loadCtx.Err(); err != nil {
		return nil, err
	}
	var client any
	var err error
	if p.loaderWithBroker != nil {
		client, err = p.loaderWithBroker(loaderCtx, wrapGRPCBroker(broker), conn)
	} else if p.loader != nil {
		client, err = p.loader(loaderCtx, conn)
	} else {
		client = conn
	}
	return client, err
}

func defaultGRPCPreset(ctx context.Context, cfg *GRPCConfig) map[string]hcplugin.Plugin {
	return map[string]hcplugin.Plugin{
		defaultGRPCPresetPluginName: &grpcPresetPlugin{
			loader:           cfg.Loader,
			loaderWithBroker: cfg.LoaderWithBroker,
			loadCtx:          ctx,
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
