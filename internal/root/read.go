package root

import (
	"io"
	"io/fs"
	"path"

	"mdlink/internal/diagnostic"
)

const MaxBodySize int64 = 64 << 20

func ReadMarkdown(file File) ([]byte, *diagnostic.Diagnostic) {
	if path.Ext(file.LogicalPath) != ".md" {
		return nil, nil
	}
	return readFile(file)
}

func readFile(file File) ([]byte, *diagnostic.Diagnostic) {
	failure := &diagnostic.Diagnostic{Code: "unreadable-file", Source: file.LogicalPath}
	root, err := openCheckedRoot(file.rootPath, file.rootIdentity)
	if err != nil {
		return nil, failure
	}
	defer root.Close()
	input, err := openRegular(root, file.relativePath)
	if err != nil {
		return nil, failure
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, failure
	}
	id, err := physicalID(info, file.PhysicalPath)
	if err != nil || id != file.physicalIdentity {
		return nil, failure
	}
	body, code := readStableBody(input, info)
	if code != "" {
		return nil, &diagnostic.Diagnostic{Code: code, Source: file.LogicalPath}
	}
	return body, nil
}

func readStableBody(input fs.File, before fs.FileInfo) ([]byte, string) {
	body, err := readBody(input)
	if err != nil {
		return nil, "unreadable-file"
	}
	if int64(len(body)) > MaxBodySize {
		return nil, "file-too-large"
	}
	after, err := input.Stat()
	if err != nil {
		return nil, "unreadable-file"
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return nil, "changed-during-read"
	}
	return body, ""
}

func readBody(input io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(input, MaxBodySize+1))
}
