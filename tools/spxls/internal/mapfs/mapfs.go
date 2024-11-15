package mapfs

import (
	"io"
	"io/fs"
	"path"
	"strings"
	"time"
)

// GetFileMapFunc is the type for function that returns a map of files.
type GetFileMapFunc func() map[string][]byte

// MapFS implements [fs.ReadDirFS] using a map of files.
type MapFS struct {
	getFileMap GetFileMapFunc
}

// New creates a new virtual file system.
func New(getFileMap GetFileMapFunc) *MapFS {
	return &MapFS{
		getFileMap: getFileMap,
	}
}

// Open implements [fs.ReadDirFS].
func (mfs *MapFS) Open(name string) (fs.File, error) {
	fileMap := mfs.getFileMap()

	name = path.Clean("/" + name)[1:] // Normalize path.
	if name == "" {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	content, ok := fileMap[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return &file{
		name:    name,
		content: content,
	}, nil
}

// ReadDir implements [fs.ReadDirFS].
func (mfs *MapFS) ReadDir(name string) ([]fs.DirEntry, error) {
	fileMap := mfs.getFileMap()

	name = path.Clean("/" + name)[1:] // Normalize path.
	if name == "" {
		name = "."
	}
	if name != "." {
		// Check if directory exists by looking for files with this prefix.
		var hasPrefix bool
		for p := range fileMap {
			if strings.HasPrefix(p, name+"/") {
				hasPrefix = true
				break
			}
		}
		if !hasPrefix {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
		}
	}

	dirs := map[string]struct{}{}
	files := map[string]struct{}{}
	prefix := ""
	if name != "." {
		prefix = name + "/"
	}
	for p := range fileMap {
		if !strings.HasPrefix(p, prefix) {
			continue
		}

		relPath := p[len(prefix):]
		parts := strings.Split(relPath, "/")
		if len(parts) == 1 {
			// It's a file in the current directory.
			files[parts[0]] = struct{}{}
		} else if len(parts) > 1 && parts[0] != "" {
			// It's a subdirectory.
			dirs[parts[0]] = struct{}{}
		}
	}

	entries := make([]fs.DirEntry, 0, len(dirs)+len(files))
	for d := range dirs {
		entries = append(entries, &dirEntry{
			name:  d,
			isDir: true,
		})
	}
	for f := range files {
		entries = append(entries, &dirEntry{
			name:  f,
			isDir: false,
			size:  int64(len(fileMap[prefix+f])),
		})
	}
	return entries, nil
}

// file implements [fs.file].
type file struct {
	name    string
	content []byte
	offset  int64
}

// Stat implements [fs.File].
func (f *file) Stat() (fs.FileInfo, error) {
	return &fileInfo{
		name:    path.Base(f.name),
		size:    int64(len(f.content)),
		mode:    0444,
		modTime: time.Now(),
		isDir:   false,
	}, nil
}

// Read implements [fs.File].
func (f *file) Read(b []byte) (int, error) {
	if f.offset >= int64(len(f.content)) {
		return 0, io.EOF
	}

	n := copy(b, f.content[f.offset:])
	f.offset += int64(n)
	return n, nil
}

// Close implements [fs.File].
func (f *file) Close() error {
	return nil
}

// fileInfo implements [fs.fileInfo].
type fileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *fileInfo) Name() string       { return fi.name }
func (fi *fileInfo) Size() int64        { return fi.size }
func (fi *fileInfo) Mode() fs.FileMode  { return fi.mode }
func (fi *fileInfo) ModTime() time.Time { return fi.modTime }
func (fi *fileInfo) IsDir() bool        { return fi.isDir }
func (fi *fileInfo) Sys() any           { return nil }

// dirEntry implements [fs.dirEntry].
type dirEntry struct {
	name  string
	isDir bool
	size  int64
}

func (de *dirEntry) Name() string { return de.name }
func (de *dirEntry) IsDir() bool  { return de.isDir }
func (de *dirEntry) Type() fs.FileMode {
	mode := fs.FileMode(0444)
	if de.isDir {
		mode |= fs.ModeDir
	}
	return mode
}

func (de *dirEntry) Info() (fs.FileInfo, error) {
	return &fileInfo{
		name:    de.name,
		size:    de.size,
		mode:    de.Type(),
		modTime: time.Now(),
		isDir:   de.isDir,
	}, nil
}
