package render

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestMergedFS_FirstFSWins(t *testing.T) {
	a := fstest.MapFS{"file.txt": {Data: []byte("a")}}
	b := fstest.MapFS{"file.txt": {Data: []byte("b")}}

	m := NewMergedFS(a, b)
	data, err := fs.ReadFile(m, "file.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "a" {
		t.Fatalf("got %q, want %q", string(data), "a")
	}
}

func TestMergedFS_FallsBack(t *testing.T) {
	a := fstest.MapFS{}
	b := fstest.MapFS{"file.txt": {Data: []byte("b")}}

	m := NewMergedFS(a, b)
	data, err := fs.ReadFile(m, "file.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "b" {
		t.Fatalf("got %q, want %q", string(data), "b")
	}
}

func TestMergedFS_NotFound(t *testing.T) {
	a := fstest.MapFS{"a.txt": {}}
	b := fstest.MapFS{"b.txt": {}}

	m := NewMergedFS(a, b)
	_, err := fs.ReadFile(m, "missing.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want ErrNotExist", err)
	}
}

func TestMergedFS_ErrorPropagation(t *testing.T) {
	errSentinel := errors.New("permission denied")
	failFS := failFS{err: errSentinel}
	okFS := fstest.MapFS{"ignored": {}}

	m := NewMergedFS(failFS, okFS)
	_, err := m.Open("anything")
	if !errors.Is(err, errSentinel) {
		t.Fatalf("error = %v, want %v", err, errSentinel)
	}
}

type failFS struct{ err error }

func (f failFS) Open(name string) (fs.File, error) {
	return nil, f.err
}

func TestMergedFS_Empty(t *testing.T) {
	m := NewMergedFS()
	_, err := m.Open("anything")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want ErrNotExist", err)
	}
}
