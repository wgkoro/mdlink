package root

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"mdlink/internal/diagnostic"
)

type File struct {
	LogicalPath      string
	PhysicalPath     string
	physicalIdentity string
	rootPath         string
	rootIdentity     string
	relativePath     string
}

type Options struct {
	NoGitignore    bool
	Exclude        []string
	FollowSymlinks []string
}

var ErrInvalidOptions = errors.New("invalid root options")

func Walk(directory string, options Options) ([]File, []diagnostic.Diagnostic, error) {
	excludes := make([]string, 0, len(options.Exclude))
	for _, value := range options.Exclude {
		clean, err := CleanLogicalPath(value)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: invalid exclude: %v", ErrInvalidOptions, err)
		}
		excludes = append(excludes, clean)
	}
	follow := make(map[string]bool, len(options.FollowSymlinks))
	for _, value := range options.FollowSymlinks {
		clean, err := CleanLogicalPath(value)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: invalid follow-symlink: %v", ErrInvalidOptions, err)
		}
		follow[clean] = false
		for _, excluded := range excludes {
			if withinPath(clean, excluded) {
				follow[clean] = true
			}
		}
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("root must be a directory")
	}
	rootID, err := physicalID(info, directory)
	if err != nil {
		return nil, nil, err
	}
	mdlinkRules, failure := readIgnore(directory, rootID, ".", ".", ".mdlinkignore")
	if failure != nil {
		return nil, []diagnostic.Diagnostic{*failure}, fmt.Errorf("cannot read mdlinkignore %q", failure.Source)
	}
	var files []File
	var diagnostics []diagnostic.Diagnostic
	visited := make(map[string]string)
	var visit func(string, string, string, string, []ignoreRule) error
	visit = func(physical, logical, originPath, originID string, rules []ignoreRule) error {
		for _, excluded := range excludes {
			if withinPath(logical, excluded) {
				return nil
			}
		}
		info, failure := inspectFile(physical, logical)
		if failure != nil {
			if logical == "." {
				return fmt.Errorf("cannot inspect root directory")
			}
			diagnostics = append(diagnostics, *failure)
			return nil
		}
		if logical != "." && (ignoredBy(rules, logical, info.IsDir()) || ignoredBy(mdlinkRules, logical, info.IsDir())) {
			for value := range follow {
				if withinPath(value, logical) {
					follow[value] = true
				}
			}
			return nil
		}
		var err error
		followed := false
		if info.Mode()&os.ModeSymlink != 0 {
			_, allowed := follow[logical]
			if !allowed {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Code: "skipped-symlink", Source: logical})
				return nil
			}
			follow[logical] = true
			followed = true
			physical, err = filepath.EvalSymlinks(physical)
			if err != nil {
				return fmt.Errorf("cannot resolve allowed symlink %q", logical)
			}
			info, err = os.Stat(physical)
			if err != nil {
				return fmt.Errorf("cannot stat allowed symlink %q", logical)
			}
		}
		if info.IsDir() {
			switch path.Base(logical) {
			case ".obsidian", ".git", ".trash":
				return nil
			}
		}
		var id string
		if info.IsDir() || info.Mode().IsRegular() {
			id, err = physicalID(info, physical)
			if err != nil {
				return fmt.Errorf("cannot identify %q", logical)
			}
			if previous, exists := visited[id]; exists && previous != logical {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Code: "duplicate-physical-file", Source: logical})
				return fmt.Errorf("duplicate physical file at %q and %q", previous, logical)
			}
			visited[id] = logical
		}
		if info.IsDir() {
			if followed {
				originPath, originID = physical, id
			}
			relative, err := filepath.Rel(originPath, physical)
			if err != nil {
				return fmt.Errorf("cannot locate directory %q", logical)
			}
			entries, err := readDirectory(originPath, originID, relative, id)
			if err != nil {
				if logical == "." {
					return fmt.Errorf("cannot read root directory")
				}
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Code: "unreadable-file", Source: logical})
				return nil
			}
			if !options.NoGitignore {
				local, failure := readIgnore(originPath, originID, relative, logical, ".gitignore")
				if failure != nil {
					diagnostics = append(diagnostics, *failure)
					return fmt.Errorf("cannot read gitignore %q", failure.Source)
				}
				rules = append(rules[:len(rules):len(rules)], local...)
			}
			for _, entry := range entries {
				child, err := CleanLogicalPath(path.Join(logical, entry.Name()))
				if err != nil {
					return err
				}
				if err := visit(filepath.Join(physical, entry.Name()), child, originPath, originID, rules); err != nil {
					return err
				}
			}
		} else if info.Mode().IsRegular() {
			if followed {
				originPath = filepath.Dir(physical)
				parentInfo, err := os.Stat(originPath)
				if err != nil {
					return fmt.Errorf("cannot stat allowed file parent %q", logical)
				}
				originID, err = physicalID(parentInfo, originPath)
				if err != nil {
					return fmt.Errorf("cannot identify allowed file parent %q", logical)
				}
			}
			relative, err := filepath.Rel(originPath, physical)
			if err != nil {
				return fmt.Errorf("cannot locate file %q", logical)
			}
			files = append(files, File{LogicalPath: logical, PhysicalPath: physical, physicalIdentity: id, rootPath: originPath, rootIdentity: originID, relativePath: relative})
		}
		return nil
	}
	err = visit(directory, ".", directory, rootID, nil)
	if err == nil {
		for value, found := range follow {
			if !found {
				return nil, diagnostics, fmt.Errorf("%w: follow-symlink %q was not reached as an allowed symlink", ErrInvalidOptions, value)
			}
		}
	}
	if err != nil {
		return nil, diagnostics, err
	}
	return files, diagnostics, nil
}

func CleanLogicalPath(value string) (string, error) {
	if value == "" || path.IsAbs(value) || strings.ContainsRune(value, 0) {
		return "", fmt.Errorf("invalid logical path")
	}
	value = path.Clean(value)
	if value == "." || value == ".." || strings.HasPrefix(value, "../") {
		return "", fmt.Errorf("logical path leaves root")
	}
	return value, nil
}

func withinPath(value, parent string) bool {
	return value == parent || strings.HasPrefix(value, parent+"/")
}

func inspectFile(physical, logical string) (os.FileInfo, *diagnostic.Diagnostic) {
	info, err := os.Lstat(physical)
	if err != nil {
		return nil, &diagnostic.Diagnostic{Code: "unreadable-file", Source: logical}
	}
	return info, nil
}
