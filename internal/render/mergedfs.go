package render

import (
	"errors"
	"io/fs"
)

// MergedFS presents multiple fs.FS instances as a single virtual filesystem.
// Open(name) tries each fs in order and returns the first successful result.
// If no fs has the file, fs.ErrNotExist is returned.
// Any non-ErrNotExist error is returned immediately.
type MergedFS []fs.FS

// NewMergedFS builds a MergedFS from one or more filesystems. The first fsys
// has highest priority; the last is tried last.
func NewMergedFS(fsys ...fs.FS) MergedFS {
	return MergedFS(fsys)
}

// Open implements fs.FS.
func (m MergedFS) Open(name string) (fs.File, error) {
	for _, f := range m {
		file, err := f.Open(name)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return nil, fs.ErrNotExist
}
