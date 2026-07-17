package goplugin

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func (m *Manager) loadWASM(ctx context.Context, info Info, pluginRoot string) (backendLoadResult, error) {
	if m.cfg.WASM == nil {
		return backendLoadResult{}, fmt.Errorf("wasm backend config is not set")
	}
	if m.cfg.WASM.Loader == nil {
		return backendLoadResult{}, fmt.Errorf("wasm Loader is required")
	}

	modulePath, err := resolveWASMModulePath(pluginRoot, info.Command)
	if err != nil {
		return backendLoadResult{}, err
	}

	clientCfg := defaultWASMClientConfig()
	if m.cfg.WASM.ClientConfigOverride != nil {
		m.cfg.WASM.ClientConfigOverride(clientCfg)
	}
	if clientCfg.NewRuntime == nil {
		return backendLoadResult{}, fmt.Errorf("wasm NewRuntime is required")
	}
	if clientCfg.ModuleConfig == nil {
		return backendLoadResult{}, fmt.Errorf("wasm ModuleConfig is required")
	}

	client, cleanup, err := m.cfg.WASM.Loader(ctx, modulePath, info, clientCfg)
	if err != nil {
		return backendLoadResult{}, err
	}
	return backendLoadResult{
		client:  client,
		cleanup: cleanup,
	}, nil
}

func defaultWASMClientConfig() *WASMClientConfig {
	return &WASMClientConfig{
		NewRuntime: func(ctx context.Context) (wazero.Runtime, error) {
			r := wazero.NewRuntime(ctx)
			if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
				_ = r.Close(ctx)
				return nil, err
			}
			return r, nil
		},
		ModuleConfig: wazero.NewModuleConfig().WithStartFunctions("_initialize"),
	}
}

func resolveWASMModulePath(pluginRoot, command string) (string, error) {
	commandLine := strings.ReplaceAll(command, "$PLUGIN_ROOT", pluginRoot)
	args, err := splitCommand(commandLine)
	if err != nil {
		return "", fmt.Errorf("parse wasm command: %w", err)
	}
	modulePath := args[0]
	modulePath = filepath.Clean(modulePath)
	if !filepath.IsAbs(modulePath) {
		prefix := pluginRoot + string(filepath.Separator)
		if strings.HasPrefix(modulePath, prefix) || modulePath == pluginRoot {
			return modulePath, nil
		}
		modulePath = filepath.Join(pluginRoot, modulePath)
	}
	return modulePath, nil
}
