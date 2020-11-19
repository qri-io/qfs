package qfs

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	logger "github.com/ipfs/go-log"
)

var (
	log = logger.Logger("qfs")
	// ErrNotFound is the canonical error for not finding a value
	ErrNotFound = errors.New("path not found")
	// ErrReadOnly is a sentinel value for Filesystems that aren't writable
	ErrReadOnly = errors.New("readonly filesystem")
)

// PathResolver is the "get" portion of a Filesystem
type PathResolver interface {
	Get(ctx context.Context, path string) (File, error)
}

// Filesystem abstracts & unifies filesystem-like behaviour
type Filesystem interface {
	// Type returns a string identifier that distinguishes a filesystem from
	// all other implementations, example identifiers include: "local", "ipfs",
	// and "http"
	// types are used as path prefixes when multiplexing filesystems
	Type() string
	// Has returns whether the `path` is mapped to a value.
	// In some contexts, it may be much cheaper only to check for existence of
	// a value, rather than retrieving the value itself. (e.g. HTTP HEAD).
	// The default implementation is found in `GetBackedHas`.
	Has(ctx context.Context, path string) (exists bool, err error)
	// Get fetching files and directories from path strings.
	// in practice path strings can be things like:
	// * a local filesystem
	// * URLS (a "URL path resolver") or
	// * content-addressed file systems like IPFS or Git
	// Datasets & dataset components use a filesource to resolve string references
	Get(ctx context.Context, path string) (File, error)
	// Put places a file or directory on the filesystem, returning the root path.
	// The returned path may or may not honor the path of the given file
	Put(ctx context.Context, file File) (path string, err error)
	// Delete removes a file or directory from the filesystem
	Delete(ctx context.Context, path string) (err error)
}

// Config binds a filesystem type to a configuration map
type Config struct {
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config,omitempty"`
}

// Constructor is a function that creates a filesystem from a config map
// the passed in context should last for the duration of the existence of the
// store. Any resources allocated by the store should be scoped to this context
type Constructor func(ctx context.Context, cfg map[string]interface{}) (Filesystem, error)

// ReleasingFilesystem provides a channel to signal cleanup is finished. It
// sends after a filesystem has closed & about to release all it's resources
type ReleasingFilesystem interface {
	Filesystem
	Done() <-chan struct{}
	DoneErr() error
}

// Destroyer is an optional interface to tear down a filesystem, removing all
// persisted resources
type Destroyer interface {
	Destroy() error
}

// PinningFS interface for content stores that support the concept of pinnings
type PinningFS interface {
	Pin(ctx context.Context, key string, recursive bool) error
	Unpin(ctx context.Context, key string, recursive bool) error
}

// CAFS stands for "content-addressed filesystem". Filesystem that implement
// this interface declare that  all paths to persisted content are reference-by
// -hash.
// TODO (b5) - write up a spec test suite for CAFS conformance
type CAFS interface {
	IsContentAddressedFilesystem()
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
	} else if strings.HasPrefix(path, "/ipfs") {
		return "ipfs"
	} else if strings.HasPrefix(path, "/mem") {
		return "mem"
	} else if strings.HasPrefix(path, "/map") {
		return "map"
	}
	return "local"
}
