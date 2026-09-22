package app

import (
	"errors"
	"os"
	"path/filepath"
)

var errInvalidRoot = errors.New("root must be an existing directory")

func rootDirectory(value string, explicit bool) (string, error) {
	if explicit && value == "" {
		return "", errInvalidRoot
	}
	if !explicit {
		value = os.Getenv("MDLINK_ROOT")
	}
	if value == "" {
		current, err := os.Getwd()
		if err != nil {
			return "", err
		}
		for {
			info, err := os.Stat(filepath.Join(current, ".obsidian"))
			if err == nil && info.IsDir() {
				value = current
				break
			}
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return "", err
			}
			parent := filepath.Dir(current)
			if parent == current {
				return "", errInvalidRoot
			}
			current = parent
		}
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errInvalidRoot
	}
	return canonical, nil
}
