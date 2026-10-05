package goplugin

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func preparePluginDir(ctx context.Context, dir, tempDir string, copyToTemp bool) (tmpRoot string, info Info, pluginRoot string, err error) {
	source, err := filepath.Abs(dir)
	if err != nil {
		return "", Info{}, "", fmt.Errorf("resolve plugin directory: %w", err)
	}
	st, err := os.Stat(source)
	if err != nil {
		return "", Info{}, "", fmt.Errorf("stat plugin directory: %w", err)
	}
	if !st.IsDir() {
		return "", Info{}, "", fmt.Errorf("plugin path %q is not a directory", dir)
	}
	root := source
	if copyToTemp {
		// Check before creating the destination, which may be inside source.
		// WalkDir does not follow symlinks; only ordinary files and directories
		// are supported for copying.
		if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 || (!entry.IsDir() && !entry.Type().IsRegular()) {
				return fmt.Errorf("unsupported plugin file %q: only regular files and directories can be copied", path)
			}
			return nil
		}); err != nil {
			return "", Info{}, "", err
		}
		tmpRoot, err = os.MkdirTemp(tempDir, "plg-*")
		if err != nil {
			return "", Info{}, "", fmt.Errorf("create temp dir: %w", err)
		}
		defer func() {
			if err != nil {
				err = rollbackTempDir(tmpRoot, err)
			}
		}()
		root, err = filepath.Abs(tmpRoot)
		if err != nil {
			return tmpRoot, Info{}, "", fmt.Errorf("resolve temp dir: %w", err)
		}
		// Resolve symlinks in the roots so an aliased temp directory cannot
		// cause the recursive copy to copy its own output.
		realSource, resolveErr := filepath.EvalSymlinks(source)
		if resolveErr != nil {
			return tmpRoot, Info{}, "", resolveErr
		}
		realRoot, resolveErr := filepath.EvalSymlinks(root)
		if resolveErr != nil {
			return tmpRoot, Info{}, "", resolveErr
		}
		err = copyPluginDir(ctx, realSource, realRoot)
		if err != nil {
			return tmpRoot, Info{}, "", err
		}
	}
	info, err = ReadInfo(filepath.Join(root, "info.yaml"))
	if err != nil {
		return tmpRoot, Info{}, "", err
	}
	pluginRoot = filepath.Join(root, "Content")
	st, err = os.Stat(pluginRoot)
	if err != nil {
		return tmpRoot, Info{}, "", fmt.Errorf("stat plugin content directory: %w", err)
	}
	if !st.IsDir() {
		return tmpRoot, Info{}, "", fmt.Errorf("plugin Content path is not a directory")
	}
	return tmpRoot, info, pluginRoot, nil
}

func copyPluginDir(ctx context.Context, source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err != nil {
			return err
		}
		if path == destination {
			return filepath.SkipDir
		}
		if entry.Type()&os.ModeSymlink != 0 || (!entry.IsDir() && !entry.Type().IsRegular()) {
			return fmt.Errorf("unsupported plugin file %q: only regular files and directories can be copied", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyPluginFile(ctx, path, target)
	})
}

func copyPluginFile(ctx context.Context, source, target string) error {
	src, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open plugin file: %w", err)
	}
	defer src.Close()
	st, err := src.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("plugin file %q is not a regular file", source)
	}
	dst, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, st.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create copied plugin file: %w", err)
	}
	_, copyErr := io.Copy(dst, contextReader{ctx: ctx, r: src})
	// Apply permissions explicitly so the umask does not strip executable bits.
	if copyErr == nil {
		copyErr = dst.Chmod(st.Mode().Perm())
	}
	closeErr := dst.Close()
	if copyErr != nil {
		return fmt.Errorf("copy plugin file %q: %w", source, copyErr)
	}
	return closeErr
}
