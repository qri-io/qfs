package qfs

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	// ErrNotFound is the canonical error for not finding a file
	ErrNotFound = errors.New("file not found")
	// ErrReadOnly is a sentinel value for Filesystems that aren't writable
	ErrReadOnly = errors.New("readonly filesystem")
)

// PathResolver is the "get" portion of a Filesystem
type PathResolver interface {
	Get(ctx context.Context, path string) (File, error)
}

// Filesystem abstracts & unifies filesystem-like behaviour
type Filesystem interface {
	// FSName returns a string identifier that distinguishes a filesystem from
	// all other implementations, example identifiers include: "local", "ipfs",
	// and "http"
	// names are used as path prefixes on files written to the FS
	FSName() string

	Has(ctx context.Context, name string) (bool, error)

	// Get fetching files and directories from path strings.
	// in practice path strings can be things like:
	// * a local filesystem
	// * URLS (a "URL path resolver") or
	// * content-addressed file systems like IPFS or Git
	// Datasets & dataset components use a filesource to resolve string references
	OpenFile(ctx context.Context, name string) (File, error)

	// OpenEncryptedFile(ctx context.Context, name string, privKey crypto.PrivKey, nonce [32]byte) (File, error)

	// Put places a file or directory on the filesystem, returning the root path.
	// The returned path may or may not honor the path of the given file
	WriteFile(ctx context.Context, file File) (name string, err error)
	// WriteEncryptedFile(ctx context.Context, file File, privKey crypto.PrivKey, nonce [32]byte) (name string, err error)
	// Delete removes a file or directory from the filesystem
	Delete(ctx context.Context, path string) (err error)
}

// NamePrefix returns a string prefix for a given filesystem. Prefixes are the
// filesystem name padded by forward slashes
func NamePrefix(fs Filesystem) string {
	return fmt.Sprintf("/%s/", fs.FSName())
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

// ReleasingFS provides a channel to signal cleanup is finished. It
// sends after a filesystem has closed & about to release all it's resources
type ReleasingFS interface {
	Filesystem
	Done() <-chan struct{}
	DoneErr() error
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
	} else if strings.HasPrefix(path, "/ipfs") {
		return "ipfs"
	} else if strings.HasPrefix(path, "/mem") {
		return "mem"
	} else if strings.HasPrefix(path, "/map") {
		return "map"
	}
	return "local"
}
