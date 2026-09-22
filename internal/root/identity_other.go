//go:build !darwin && !linux

package root

import (
	"os"
	"path/filepath"
)

func physicalID(_ os.FileInfo, physical string) (string, error) {
	absolute, err := filepath.Abs(physical)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}
