package qfs

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/qri-io/value"
)

var (
	// ErrNotFound is the canonical error for not finding a value
	ErrNotFound = errors.New("path not found")
	// ErrReadOnly is a sentinel value for Filesystems that aren't writable
	ErrReadOnly = errors.New("readonly filesystem")
)

// Filesystem abstracts & unifies filesystem-like behaviour with qri values
//
// A traditional file system is generally composed of three types:
//   * file - resolve to a stream of bytes
//   * directories - a colletion of one of the three types
//   * symlinks - a pointer to one of the three types
// a qfs Filesystem instead works with Files, which are composed of qri values
// qri values extend byte streams into structured data
// filesystems by default are read-only
type Filesystem interface {
	value.Resolver
	// Get fetching files and directories from path strings.
	// in practice path strings can be things like:
	// * a local filesystem
	// * URLS (a "URL path resolver") or
	// * content-addressed file systems like IPFS or Git
	Get(ctx context.Context, path string) (File, error)
	// Put places a file or directory on the filesystem, returning the root path.
	// The returned path may or may not honor the path of the given file
	Put(ctx context.Context, file File) (path string, err error)
	// Delete removes a file or directory from the filesystem
	Delete(ctx context.Context, path string) (err error)
}

// WritableFilesystem is a Filsystem that supports editing
type WritableFilesystem interface {
	Filesystem
	// // Put places a file or directory on the filesystem, returning the root path.
	// // The returned path may or may not honor the path of the given file
	// Put(ctx context.Context, file File) (path string, err error)
	// // Delete removes a file or directory from the filesystem
	// Delete(ctx context.Context, path string) (err error)
}

// Destroyer is an optional interface to tear down a filesystem, removing all
// persisted resources
type Destroyer interface {
	Destroy() error
}

// AbsPath adjusts the provided string to a path lib functions can work with
// because paths for Qri can come from the local filesystem, an http url, or
// the distributed web, Absolutizing is a little tricky
//
// If lib in put params call for a path, running input through AbsPath before
// calling a lib function should help reduce errors. calling AbsPath on empty
// string has no effect
func AbsPath(path *string) (err error) {
	if *path == "" {
		return
	}

	*path = strings.TrimSpace(*path)
	p := *path

	// bail on urls and ipfs hashes
	pk := PathKind(p)
	if pk == "http" || pk == "ipfs" {
		return
	}

	// TODO (b5) - perform tilda (~) expansion
	if filepath.IsAbs(p) {
		return
	}
	*path, err = filepath.Abs(p)
	return
}

// PathKind estimates what type of resolver string path is referring to
func PathKind(path string) string {
	if path == "" {
		return "none"
	} else if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return "http"
	} else if strings.HasPrefix(path, "/ipfs") || strings.HasPrefix(path, "/ipld") {
		return "ipfs"
	} else if strings.HasPrefix(path, "/map") || strings.HasPrefix(path, "/cafs") {
		return "cafs"
	}
	return "local"
}
