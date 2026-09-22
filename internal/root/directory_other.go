//go:build !darwin && !linux

package root

import (
	"fmt"
	"os"
)

func openDirectory(_ *os.Root, _ string) (*os.File, error) {
	return nil, fmt.Errorf("secure directory open is unsupported on this platform")
}
