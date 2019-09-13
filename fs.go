package qfs

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

var (
	// ErrNotFound is the canonical error for not finding a value
	ErrNotFound = errors.New("path not found")
)

// PathResolver is an interface for fetching files and directories from path strings.
// in practice path strings can be things like:
// * a local filesystem
// * URLS (a "URL path resolver") or
// * content-addressed file systems like IPFS or Git
// Datasets & dataset components use a filesource to resolve string references
type PathResolver interface {
	Get(ctx context.Context, path string) (File, error)
}

// Filesystem abstracts & unifies filesystem-like behaviour
// For now it's just a wrapper around PathResolver, but it'll expand once we merge
// write-like functionality from cafs
type Filesystem interface {
	PathResolver
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

	// TODO - perform tilda (~) expansion
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
	} else if strings.HasPrefix(path, "/ipfs") {
		return "ipfs"
	} else if strings.HasPrefix(path, "/map") || strings.HasPrefix(path, "/cafs") {
		return "cafs"
	}
	return "local"
}
