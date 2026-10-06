package goplugin

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Manager struct {
	cfg Config
}

func NewManager(cfg Config) (*Manager, error) {
	cfg.defaults()
	if cfg.GRPC == nil && cfg.WASM == nil {
		return nil, fmt.Errorf("at least one backend config is required")
	}
	return &Manager{cfg: cfg}, nil
}

func (m *Manager) Load(path string) (*Handle, error) {
	return m.LoadContext(context.Background(), path)
}

// LoadContext loads a package using ctx for preparation and backend startup.
// Cancellation after a successful load does not close the plugin.
// A failed rollback returns a RollbackError retaining its handle for cleanup retries.
func (m *Manager) LoadContext(ctx context.Context, path string) (*Handle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tmpRoot, info, pluginRoot, err := extractPlugin(ctx, path, m.cfg.TempDir)
	if err != nil {
		return nil, err
	}

	return m.loadPrepared(ctx, info, path, pluginRoot, tmpRoot)
}

// LoadDir loads a directory containing info.yaml and Content.
// If copyToTemp is true, it copies the directory to a private temporary directory
// that is removed on failure or unload. Otherwise the source directory is used
// directly and is never removed by the manager.
func (m *Manager) LoadDir(dir string, copyToTemp bool) (*Handle, error) {
	return m.LoadDirContext(context.Background(), dir, copyToTemp)
}

// LoadDirContext is LoadDir with cancellation during preparation and startup.
func (m *Manager) LoadDirContext(ctx context.Context, dir string, copyToTemp bool) (*Handle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tmpRoot, info, pluginRoot, err := preparePluginDir(ctx, dir, m.cfg.TempDir, copyToTemp)
	if err != nil {
		return nil, err
	}
	return m.loadPrepared(ctx, info, dir, pluginRoot, tmpRoot)
}

func (m *Manager) loadPrepared(ctx context.Context, info Info, source, pluginRoot, tmpRoot string) (*Handle, error) {
	var loadRes backendLoadResult
	err := ctx.Err()
	if err == nil {
		loadRes, err = m.loadByType(ctx, info, pluginRoot)
	}
	if ctxErr := ctx.Err(); ctxErr != nil && !errors.Is(err, ctxErr) {
		err = errors.Join(err, ctxErr)
	}

	h := &Handle{
		client:   loadRes.client,
		info:     info,
		plugin:   source,
		root:     pluginRoot,
		tmpRoot:  tmpRoot,
		cleanup:  loadRes.cleanup,
		exit:     loadRes.exit,
		unloader: removeDir,
	}
	if err != nil {
		// Rollback must still run when the startup context has been cancelled.
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if closeErr := h.Close(rollbackCtx); closeErr != nil {
			if tmpRoot != "" {
				closeErr = fmt.Errorf("rollback plugin (temporary directory %q retained): %w", tmpRoot, closeErr)
			} else {
				closeErr = fmt.Errorf("rollback plugin %q: %w", source, closeErr)
			}
			return nil, &RollbackError{Handle: h, Cause: errors.Join(err, closeErr)}
		}
		return nil, err
	}
	return h, nil
}

func (m *Manager) Unload(h *Handle) error {
	return m.UnloadContext(context.Background(), h)
}

// UnloadContext closes one handle. The application owns loaded handles and must
// stop using each plugin before unloading it.
func (m *Manager) UnloadContext(ctx context.Context, h *Handle) error {
	if h == nil {
		return nil
	}
	return h.Close(ctx)
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

// RollbackError retains a backend whose startup rollback did not finish.
// The caller must clean Handle before loading a replacement instance.
type RollbackError struct {
	Handle *Handle
	Cause  error
}

func (e *RollbackError) Error() string { return e.Cause.Error() }
func (e *RollbackError) Unwrap() error { return e.Cause }
