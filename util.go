package goplugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func splitCommand(cmd string) ([]string, error) {
	fields := strings.Fields(strings.TrimSpace(cmd))
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	return fields, nil
}

func removeDir(path string) error {
	if path == "" {
		return nil
	}
	return os.RemoveAll(path)
}

func rollbackTempDir(root string, cause error) error {
	if err := removeDir(root); err != nil {
		return errors.Join(cause, fmt.Errorf("remove plugin temporary directory %q: %w", root, err))
	}
	return cause
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
