package qfs

import (
	"context"
	"fmt"

	"github.com/qri-io/value"
)

// NewMux creates a new path muxer
func NewMux(handlers map[string]Filesystem) *Mux {
	return &Mux{handlers: handlers}
}

// Mux multiplexes together multiple filesystems using path multiplexing.
// It's a way to use multiple filesystem implementations as a single FS
type Mux struct {
	handlers map[string]Filesystem
}

// compile-time assertion that MapStore satisfies the Filesystem interface
var _ Filesystem = (*Mux)(nil)

// SetHandler designates the resolver for a given path kind string
func (m *Mux) SetHandler(pathKind string, resolver Filesystem) {
	if m.handlers == nil {
		m.handlers = map[string]Filesystem{}
	}
	m.handlers[pathKind] = resolver
}

func noMuxerError(kind, path string) error {
	return fmt.Errorf("cannot resolve paths of kind '%s'. path: %s", kind, path)
}

// Get a path
func (m Mux) Get(ctx context.Context, path string) (File, error) {
	if path == "" {
		return nil, ErrNotFound
	}

	kind := PathKind(path)
	handler, ok := m.handlers[kind]
	if !ok {
		return nil, noMuxerError(kind, path)
	}

	return handler.Get(ctx, path)
}

func (m Mux) Resolve(ctx context.Context, l value.Link) (v value.Value, err error) {
	f, err := m.Get(ctx, l.Path())
	if err != nil {
		return nil, err
	}
	l.Resolved(f.Value())
	return f.Value(), nil
}

// Put places a file or directory on the filesystem, returning the root path.
// The returned path may or may not honor the path of the given file
func (m Mux) Put(ctx context.Context, file File) (resPath string, err error) {
	path := file.FullPath()
	kind := PathKind(path)
	handler, ok := m.handlers[kind]
	if !ok {
		return "", noMuxerError(kind, path)
	}

	return handler.Put(ctx, file)
}

// Delete removes a file or directory from the filesystem
func (m Mux) Delete(ctx context.Context, path string) (err error) {
	kind := PathKind(path)
	handler, ok := m.handlers[kind]
	if !ok {
		return noMuxerError(kind, path)
	}

	return handler.Delete(ctx, path)
}

// Demux gets a filesystem for a given path kind
func (m Mux) Demux(pathKind string) (Filesystem, error) {
	fs, ok := m.handlers[pathKind]
	if !ok {
		return nil, fmt.Errorf("no filesystem called '%s' exists", pathKind)
	}
	return fs, nil
}
