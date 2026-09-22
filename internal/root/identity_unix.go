//go:build darwin || linux

package root

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func physicalID(info os.FileInfo, physical string) (string, error) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino), nil
	}
	absolute, err := filepath.Abs(physical)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}
