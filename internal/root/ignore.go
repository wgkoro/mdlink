package root

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"mdlink/internal/diagnostic"
)

type ignoreRule struct {
	base, pattern               string
	negate, directory, anchored bool
}

func parseIgnore(body []byte, base string) ([]ignoreRule, error) {
	var rules []ignoreRule
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSuffix(line, "\r")
		for strings.HasSuffix(line, " ") {
			backslashes := 0
			for i := len(line) - 2; i >= 0 && line[i] == '\\'; i-- {
				backslashes++
			}
			if backslashes%2 == 1 {
				break
			}
			line = line[:len(line)-1]
		}
		if line == "" || line[0] == '#' {
			continue
		}
		rule := ignoreRule{base: base}
		if line[0] == '!' {
			rule.negate = true
			line = line[1:]
		}
		if line == "" {
			continue
		}
		rule.directory = strings.HasSuffix(line, "/")
		line = strings.TrimSuffix(line, "/")
		rule.anchored = strings.Contains(line, "/")
		line = strings.TrimPrefix(line, "/")
		// Git treats braces literally; preserve existing escapes when adapting them.
		var pattern strings.Builder
		inClass := false
		classStart := 0
		for i := 0; i < len(line); i++ {
			if line[i] == '\\' && i+1 < len(line) {
				pattern.WriteByte(line[i])
				i++
				pattern.WriteByte(line[i])
				continue
			}
			// Git accepts any run of two or more stars as a recursive component.
			if !inClass && line[i] == '*' && (i == 0 || line[i-1] == '/') {
				end := i
				for end < len(line) && line[end] == '*' {
					end++
				}
				if end-i >= 2 && (end == len(line) || line[end] == '/') {
					pattern.WriteString("**")
					i = end - 1
					continue
				}
			}
			if inClass && strings.HasPrefix(line[i:], "[:") {
				return nil, fmt.Errorf("unsupported POSIX character class")
			}
			if line[i] == '[' && !inClass {
				inClass = true
				classStart = i + 1
				if classStart < len(line) && (line[classStart] == '!' || line[classStart] == '^') {
					classStart++
				}
			} else if line[i] == ']' && inClass && i == classStart {
				pattern.WriteString(`\]`)
				continue
			} else if line[i] == ']' {
				inClass = false
			}
			if line[i] == '{' || line[i] == '}' {
				pattern.WriteByte('\\')
			}
			pattern.WriteByte(line[i])
		}
		rule.pattern = pattern.String()
		if strings.HasSuffix(rule.pattern, "/**") {
			rule.pattern += "/*"
		}
		if !doublestar.ValidatePattern(rule.pattern) {
			return nil, fmt.Errorf("invalid gitignore pattern")
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func ignoredBy(rules []ignoreRule, logical string, directory bool) bool {
	ignored := false
	for _, rule := range rules {
		relative := logical
		if rule.base != "." {
			if !strings.HasPrefix(logical, rule.base+"/") {
				continue
			}
			relative = strings.TrimPrefix(logical, rule.base+"/")
		}
		if rule.directory && !directory {
			continue
		}
		if !rule.anchored {
			relative = path.Base(relative)
		}
		match, _ := doublestar.Match(rule.pattern, relative)
		if match {
			ignored = !rule.negate
		}
	}
	return ignored
}

func readIgnore(originPath, originID, relative, logical, filename string) ([]ignoreRule, *diagnostic.Diagnostic) {
	name := path.Join(logical, filename)
	failure := &diagnostic.Diagnostic{Code: "unreadable-file", Source: name}
	root, err := openCheckedRoot(originPath, originID)
	if err != nil {
		return nil, failure
	}
	info, err := root.Lstat(filepath.Join(relative, filename))
	root.Close()
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, failure
	}
	if !info.Mode().IsRegular() {
		return nil, nil
	}
	physical := filepath.Join(originPath, relative, filename)
	id, err := physicalID(info, physical)
	if err != nil {
		return nil, failure
	}
	body, readFailure := readFile(File{LogicalPath: name, PhysicalPath: physical, physicalIdentity: id, rootPath: originPath, rootIdentity: originID, relativePath: filepath.Join(relative, filename)})
	if readFailure != nil {
		return nil, readFailure
	}
	rules, err := parseIgnore(body, logical)
	if err != nil {
		return nil, &diagnostic.Diagnostic{Code: "invalid-" + strings.TrimPrefix(filename, "."), Source: name}
	}
	return rules, nil
}
