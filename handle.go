package goplugin

import (
	"context"
	"sync"
)

type Handle struct {
	client any
	info   Info
	plugin string

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
