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

	unloader func(string) error
	once     sync.Once
	closeErr error
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

// RootPath returns the extracted plugin Content directory path.
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

func (h *Handle) Close(ctx context.Context) error {
	h.once.Do(func() {
		if h.cleanup != nil {
			if err := h.cleanup(ctx); err != nil {
				h.closeErr = err
				return
			}
		}
		if h.unloader != nil {
			h.closeErr = h.unloader(h.tmpRoot)
		}
	})
	return h.closeErr
}
