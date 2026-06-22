//go:build !unix

package goplugin

import (
	"fmt"
	"syscall"
)

func lookupCredential(username string) (*syscall.SysProcAttr, error) {
	return nil, fmt.Errorf("RunAsUser is unsupported on this platform (%q)", username)
}
