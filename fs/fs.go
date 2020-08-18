// Package fs is a shim for the upcoming filesystem interface for go
// not accepted yet, but Russ Cox wrote it, so it's going to go in, and if it
// doesn't, it should have.
// https://go.googlesource.com/proposal/+/master/design/draft-iofs.md
package fs

import (
	"fmt"
	"io/ioutil"
	"os"
)

// File interface representing an open file:
type File interface {
	Stat() (os.FileInfo, error)
	Read([]byte) (int, error)
	Close() error
}

// A ReadDirFile is a File that implements the ReadDir method for directory
// reading
type ReadDirFile interface {
	File
	ReadDir(n int) ([]os.FileInfo, error)
}

// FS represents a file system
type FS interface {
	Open(name string) (File, error)
}

// ReadFileFS is a filesystem with a ReadFile convenience shortcut
type ReadFileFS interface {
	FS
	ReadFile(name string) ([]byte, error)
}

// ReadFile reads an entire file from a given filesystem
func ReadFile(fsys FS, name string) ([]byte, error) {
	if fsys, ok := fsys.(ReadFileFS); ok {
		return fsys.ReadFile(name)
	}

	file, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ioutil.ReadAll(file)
}

// StatFS is a filesystem that supports a stat call
type StatFS interface {
	FS
	Stat(name string) (os.FileInfo, error)
}

// Stat gets file stats for a name on a filesystem
func Stat(fsys FS, name string) (os.FileInfo, error) {
	if fsys, ok := fsys.(StatFS); ok {
		return fsys.Stat(name)
	}

	file, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}

// ReadDirFS is a filesystem extension interface for reading an entire directory
type ReadDirFS interface {
	FS
	ReadDir(name string) ([]os.FileInfo, error)
}

// ReadDir reads an entire directory from a filesystem
func ReadDir(fsys FS, name string) ([]os.FileInfo, error) {
	if fsys, ok := fsys.(ReadDirFS); ok {
		return fsys.ReadDir(name)
	}

	file, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if file, ok := file.(ReadDirFile); ok {
		return file.ReadDir(-1)
	}

	return nil, fmt.Errorf("not a file")
}
