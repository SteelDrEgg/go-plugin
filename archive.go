package goplugin

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func extractPlugin(ctx context.Context, pluginFile, tempDir string) (tmpRoot string, info Info, pluginRoot string, err error) {
	f, err := os.Open(pluginFile)
	if err != nil {
		return "", Info{}, "", fmt.Errorf("open plugin package: %w", err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", Info{}, "", fmt.Errorf("stat plugin package: %w", err)
	}

	zr, err := zip.NewReader(f, st.Size())
	if err != nil {
		return "", Info{}, "", fmt.Errorf("read zip plugin package: %w", err)
	}

	tmpRoot, err = os.MkdirTemp(tempDir, "plg-*")
	if err != nil {
		return "", Info{}, "", fmt.Errorf("create temp dir: %w", err)
	}
	cleanupRoot := tmpRoot
	defer func() {
		if err != nil {
			err = rollbackTempDir(cleanupRoot, err)
		}
	}()
	if !filepath.IsAbs(tmpRoot) {
		absRoot, err := filepath.Abs(tmpRoot)
		if err != nil {
			return "", Info{}, "", fmt.Errorf("resolve temp dir %q: %w", tmpRoot, err)
		}
		tmpRoot = absRoot
	}

	for _, zf := range zr.File {
		if err := extractFile(ctx, tmpRoot, zf); err != nil {
			return "", Info{}, "", err
		}
	}

	info, err = ReadInfo(filepath.Join(tmpRoot, "info.yaml"))
	if err != nil {
		return "", Info{}, "", err
	}

	pluginRoot = filepath.Join(tmpRoot, "Content")
	if _, err := os.Stat(pluginRoot); err != nil {
		return "", Info{}, "", fmt.Errorf("plugin content dir missing: %w", err)
	}

	return tmpRoot, info, pluginRoot, nil
}

func extractFile(ctx context.Context, root string, zf *zip.File) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cleanName := filepath.Clean(zf.Name)
	if strings.Contains(cleanName, ".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid zip entry path %q", zf.Name)
	}
	target := filepath.Join(root, cleanName)
	if !strings.HasPrefix(target, root+string(filepath.Separator)) && target != root {
		return fmt.Errorf("invalid zip entry path %q", zf.Name)
	}

	if zf.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create dir for %q: %w", zf.Name, err)
	}

	src, err := zf.Open()
	if err != nil {
		return fmt.Errorf("open zip entry %q: %w", zf.Name, err)
	}
	defer src.Close()

	dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, zf.Mode())
	if err != nil {
		return fmt.Errorf("create extracted file %q: %w", target, err)
	}
	_, copyErr := io.Copy(dst, contextReader{ctx: ctx, r: src})
	closeErr := dst.Close()
	if copyErr != nil {
		return fmt.Errorf("extract %q: %w", zf.Name, copyErr)
	}
	return closeErr
}

func validateInfo(info Info) error {
	if info.Name == "" {
		return fmt.Errorf("info.yaml Name is required")
	}
	if info.Version == "" {
		return fmt.Errorf("info.yaml Version is required")
	}
	if info.Type != "grpc" && info.Type != "wasm" {
		return fmt.Errorf("info.yaml Type must be grpc or wasm")
	}
	if info.ContractVersion == 0 {
		return fmt.Errorf("info.yaml ContractVersion is required")
	}
	if info.Command == "" {
		return fmt.Errorf("info.yaml Command is required")
	}
	return nil
}
