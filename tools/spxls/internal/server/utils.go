package server

import (
	"io/fs"
	"strings"

	gopast "github.com/goplus/gop/ast"
)

// gopASTFileMapToSlice converts a map of [gopast.File] to a slice of [gopast.File].
func gopASTFileMapToSlice(fileMap map[string]*gopast.File) []*gopast.File {
	files := make([]*gopast.File, 0, len(fileMap))
	for _, file := range fileMap {
		files = append(files, file)
	}
	return files
}

// spxFilesFromFS returns a list of .spx files in the given file system.
func spxFilesFromFS(fs fs.ReadDirFS) ([]string, error) {
	entries, err := fs.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".spx") {
			continue
		}
		files = append(files, entry.Name())
	}
	return files, nil
}
