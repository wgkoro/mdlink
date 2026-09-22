//go:build darwin || linux

package root

import (
	"os"
	"syscall"
)

func openRegular(root *os.Root, relative string) (*os.File, error) {
	return root.OpenFile(relative, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
}
