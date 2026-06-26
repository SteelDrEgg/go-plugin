package goplugin

import (
	"context"
	"fmt"
	"sync"
)

type Manager struct {
	cfg Config

	mu sync.Mutex
}

func NewManager(cfg Config) (*Manager, error) {
	cfg.defaults()
	if cfg.TempDir == "" {
		return nil, fmt.Errorf("TempDir is required")
	}
	if cfg.GRPC == nil && cfg.WASM == nil {
		return nil, fmt.Errorf("at least one backend config is required")
	}
	return &Manager{cfg: cfg}, nil
}

func (m *Manager) Load(path string) (*Handle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tmpRoot, info, pluginRoot, err := extractPlugin(path, m.cfg.TempDir)
	if err != nil {
		return nil, err
	}

	loadRes, err := m.loadByType(context.Background(), info, pluginRoot)
	if err != nil {
		_ = removeDir(tmpRoot)
		return nil, err
	}

	h := &Handle{
		client:   loadRes.client,
		info:     info,
		plugin:   path,
		root:     pluginRoot,
		tmpRoot:  tmpRoot,
		cleanup:  loadRes.cleanup,
		unloader: removeDir,
	}
	return h, nil
}

func (m *Manager) Unload(h *Handle) error {
	if h == nil {
		return nil
	}
	return h.Close(context.Background())
}

func (m *Manager) loadByType(ctx context.Context, info Info, pluginRoot string) (backendLoadResult, error) {
	switch info.Type {
	case "grpc":
		return m.loadGRPC(ctx, info, pluginRoot)
	case "wasm":
		return m.loadWASM(ctx, info, pluginRoot)
	default:
		return backendLoadResult{}, fmt.Errorf("unsupported plugin type %q", info.Type)
	}
}
