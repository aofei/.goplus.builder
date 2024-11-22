package vfs

import (
	"io/fs"
	"path/filepath"
)

// GopParserFS wraps a [MapFS] to implement [github.com/goplus/gop/parser.FileSystem].
type GopParserFS struct {
	underlying fs.ReadDirFS
}

// NewGopParserFS creates a new Go+ parser file system.
func NewGopParserFS(underlying fs.ReadDirFS) *GopParserFS {
	return &GopParserFS{underlying: underlying}
}

// ReadDir implements [github.com/goplus/gop/parser.FileSystem].
func (gpfs *GopParserFS) ReadDir(dirname string) ([]fs.DirEntry, error) {
	return fs.ReadDir(gpfs.underlying, dirname)
}

// ReadFile implements [github.com/goplus/gop/parser.FileSystem].
func (gpfs *GopParserFS) ReadFile(filename string) ([]byte, error) {
	return fs.ReadFile(gpfs.underlying, filename)
}

// Join implements [github.com/goplus/gop/parser.FileSystem].
func (gpfs *GopParserFS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// Base implements [github.com/goplus/gop/parser.FileSystem].
func (gpfs *GopParserFS) Base(filename string) string {
	return filepath.Base(filename)
}

// Abs implements [github.com/goplus/gop/parser.FileSystem].
func (gpfs *GopParserFS) Abs(path string) (string, error) {
	return filepath.Abs(path)
}
