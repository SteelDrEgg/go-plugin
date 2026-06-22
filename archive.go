package goplugin

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func extractPlugin(pluginFile, tempDir string) (tmpRoot string, info Info, pluginRoot string, err error) {
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

	for _, zf := range zr.File {
		if err := extractFile(tmpRoot, zf); err != nil {
			_ = removeDir(tmpRoot)
			return "", Info{}, "", err
		}
	}

	infoBytes, err := os.ReadFile(filepath.Join(tmpRoot, "info.yaml"))
	if err != nil {
		_ = removeDir(tmpRoot)
		return "", Info{}, "", fmt.Errorf("read info.yaml: %w", err)
	}
	if err := yaml.Unmarshal(infoBytes, &info); err != nil {
		_ = removeDir(tmpRoot)
		return "", Info{}, "", fmt.Errorf("parse info.yaml: %w", err)
	}

	if err := validateInfo(info); err != nil {
		_ = removeDir(tmpRoot)
		return "", Info{}, "", err
	}

	pluginRoot = filepath.Join(tmpRoot, "Content")
	if _, err := os.Stat(pluginRoot); err != nil {
		_ = removeDir(tmpRoot)
		return "", Info{}, "", fmt.Errorf("plugin content dir missing: %w", err)
	}

	return tmpRoot, info, pluginRoot, nil
}

func extractFile(root string, zf *zip.File) error {
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
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("extract %q: %w", zf.Name, err)
	}

	return nil
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
