//go:build darwin || linux

package root

import (
	"os"
	"syscall"
)

func openDirectory(root *os.Root, relative string) (*os.File, error) {
	return root.OpenFile(relative, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
}
