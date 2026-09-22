//go:build !darwin && !linux

package root

import (
	"fmt"
	"os"
)

func openRegular(_ *os.Root, _ string) (*os.File, error) {
	return nil, fmt.Errorf("secure regular file open is unsupported on this platform")
}
