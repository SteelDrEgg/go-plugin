package goplugin

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

type Handle struct {
	client any
	info   Info
	plugin string
	root   string

	tmpRoot string
	cleanup func(context.Context) error

	unloader    func(string) error
	closeOnce   sync.Once
	closeGate   chan struct{}
	cleanupDone bool
	closed      bool
}

func (h *Handle) Client() any {
	return h.client
}

func (h *Handle) Info() Info {
	return h.info
}

func (h *Handle) PluginPath() string {
	return h.plugin
}

// RootPath returns the absolute path to the loaded plugin's Content directory.
func (h *Handle) RootPath() string {
	return h.root
}

// ResolvePath maps a plugin resource reference to an absolute file path under Content.
//
// The resource can be one of:
// - "/greet.txt"
// - "greet.txt"
// - "Content/greet.txt"
// - "<plugin-file>.plg/Content/greet.txt"
func (h *Handle) ResolvePath(resource string) (string, error) {
	if h.root == "" {
		return "", fmt.Errorf("plugin root is not available")
	}

	normalized := strings.TrimSpace(resource)
	if normalized == "" {
		return "", fmt.Errorf("resource path is required")
	}
	normalized = strings.ReplaceAll(normalized, "\\", "/")

	pluginFile := filepath.Base(h.plugin)
	pluginPrefix := pluginFile + "/"
	if strings.HasPrefix(normalized, pluginPrefix) {
		normalized = strings.TrimPrefix(normalized, pluginPrefix)
	}

	normalized = strings.TrimPrefix(normalized, "./")
	if strings.HasPrefix(normalized, "Content/") {
		normalized = strings.TrimPrefix(normalized, "Content/")
	}
	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return "", fmt.Errorf("resource %q escapes plugin root", resource)
		}
	}

	cleaned := path.Clean("/" + normalized)
	rel := strings.TrimPrefix(cleaned, "/")
	if rel == "" || rel == "." {
		return "", fmt.Errorf("resource %q points to plugin root", resource)
	}

	target := filepath.Join(h.root, filepath.FromSlash(rel))
	relToRoot, err := filepath.Rel(h.root, target)
	if err != nil {
		return "", fmt.Errorf("resolve resource %q: %w", resource, err)
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("resource %q escapes plugin root", resource)
	}

	return target, nil
}

// ReadFile reads bytes from a plugin resource under Content.
func (h *Handle) ReadFile(resource string) ([]byte, error) {
	target, err := h.ResolvePath(resource)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(target)
	if err != nil {
		return nil, fmt.Errorf("read plugin resource %q: %w", resource, err)
	}
	return b, nil
}

// Close releases the backend before removing private temporary resources.
// Failed steps can be retried with a new context; successful steps are not repeated.
// Concurrent Close calls are serialized and can cancel while waiting.
// The caller must stop plugin calls and resource reads before closing the handle.
func (h *Handle) Close(ctx context.Context) error {
	h.closeOnce.Do(func() {
		h.closeGate = make(chan struct{}, 1)
		h.closeGate <- struct{}{}
	})
	select {
	case <-h.closeGate:
	default:
		select {
		case <-h.closeGate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	defer func() { h.closeGate <- struct{}{} }()
	if h.closed {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !h.cleanupDone {
		if h.cleanup != nil {
			if err := h.cleanup(ctx); err != nil {
				return fmt.Errorf("close plugin backend: %w", err)
			}
		}
		h.cleanupDone = true
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if h.unloader != nil {
		if err := h.unloader(h.tmpRoot); err != nil {
			return fmt.Errorf("remove plugin temporary directory %q: %w", h.tmpRoot, err)
		}
	}
	h.closed = true
	return nil
}
