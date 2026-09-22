package root

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func readDirectory(rootPath, rootID, relative, directoryID string) ([]os.DirEntry, error) {
	root, err := openCheckedRoot(rootPath, rootID)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	directory, err := openDirectory(root, relative)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	info, err := directory.Stat()
	if err != nil {
		return nil, err
	}
	id, err := physicalID(info, filepath.Join(rootPath, relative))
	if err != nil || !info.IsDir() || id != directoryID {
		return nil, fmt.Errorf("directory identity changed")
	}
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

func openCheckedRoot(rootPath, rootID string) (*os.Root, error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	info, err := root.Stat(".")
	if err != nil {
		root.Close()
		return nil, err
	}
	id, err := physicalID(info, rootPath)
	if err != nil || id != rootID {
		root.Close()
		return nil, fmt.Errorf("read root identity changed")
	}
	return root, nil
}
