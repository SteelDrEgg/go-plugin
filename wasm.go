package goplugin

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
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

	client, cleanup, err := m.cfg.WASM.Loader(ctx, modulePath, info)
	if err != nil {
		return backendLoadResult{}, err
	}
	return backendLoadResult{
		client:  client,
		cleanup: cleanup,
	}, nil
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
