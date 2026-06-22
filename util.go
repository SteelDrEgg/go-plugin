package goplugin

import (
	"fmt"
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
