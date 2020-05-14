package muxfs

import (
	"context"
	"fmt"

	"github.com/qri-io/qfs"
	qipfs "github.com/qri-io/qfs/cafs/ipfs"
	"github.com/qri-io/qfs/httpfs"
	"github.com/qri-io/qfs/localfs"
)

// NewMux creates a new path muxer
func NewMux(handlers map[string]qfs.Filesystem) *Mux {
	return &Mux{handlers: handlers}
}

// Mux multiplexes together multiple filesystems using path multiplexing.
// It's a way to use multiple filesystem implementations as a single FS
type Mux struct {
	handlers map[string]qfs.Filesystem
}

// compile-time assertion that MapStore satisfies the Filesystem interface
var _ qfs.Filesystem = (*Mux)(nil)

// SetHandler designates the resolver for a given path kind string
func (m *Mux) SetHandler(pathKind string, resolver qfs.Filesystem) {
	if m.handlers == nil {
		m.handlers = map[string]qfs.Filesystem{}
	}
	m.handlers[pathKind] = resolver
}

// Option is a function that manipulates config details when fed to New(). Fields on
// the o parameter may be null, functions cannot assume the Config is non-null.
type Option func(o []MuxConfig) error

// MuxConfig contains the information needed to create a new filesystem
type MuxConfig struct {
	Type    string                 `json:"type"`
	Options map[string]interface{} `json:"options,omitempty"`
	Source  string                 `json:"source,omitempty"`
}

// New creates a new Mux Filesystem, if no Option funcs are provided,
// New uses a default set of Option funcs. Any Option functions passed to this
// function must check whether their fields are nil or not.
func New(ctx context.Context, cfgs []MuxConfig, opts ...Option) (*Mux, error) {
	if cfgs == nil {
		return nil, fmt.Errorf("config is required")
	}

	if len(opts) == 0 {
		// default to a standard composition of Option funcs
		opts = []Option{}
	}

	for _, opt := range opts {
		if err := opt(cfgs); err != nil {
			return nil, err
		}
	}
	mux := &Mux{}
	for _, cfg := range cfgs {
		switch cfg.Type {
		case "ipfs":
			fs, err := qipfs.NewFilestore(cfg.Options)
			if err != nil {
				return nil, err
			}
			mux.handlers["ipfs"] = fs
		case "local":
			fs, err := localfs.NewFS(cfg.Options)
			if err != nil {
				return nil, err
			}
			mux.handlers["local"] = fs
		case "http":
			fs, err := httpfs.NewFS(cfg.Options)
			if err != nil {
				return nil, err
			}
			mux.handlers["http"] = fs
		case "mem":
		case "test":
			mux.handlers["mem"] = qfs.NewMemFS()
		}
	}
	return mux, nil
}

func noMuxerError(kind, path string) error {
	return fmt.Errorf("cannot resolve paths of kind '%s'. path: %s", kind, path)
}

// Get a path
func (m Mux) Get(ctx context.Context, path string) (qfs.File, error) {
	if path == "" {
		return nil, qfs.ErrNotFound
	}

	kind := qfs.PathKind(path)
	handler, ok := m.handlers[kind]
	if !ok {
		return nil, noMuxerError(kind, path)
	}

	return handler.Get(ctx, path)
}

// Put places a file or directory on the filesystem, returning the root path.
// The returned path may or may not honor the path of the given file
func (m Mux) Put(ctx context.Context, file qfs.File) (resPath string, err error) {
	path := file.FullPath()
	kind := qfs.PathKind(path)
	handler, ok := m.handlers[kind]
	if !ok {
		return "", noMuxerError(kind, path)
	}

	return handler.Put(ctx, file)
}

// Delete removes a file or directory from the filesystem
func (m Mux) Delete(ctx context.Context, path string) (err error) {
	kind := qfs.PathKind(path)
	handler, ok := m.handlers[kind]
	if !ok {
		return noMuxerError(kind, path)
	}

	return handler.Delete(ctx, path)
}
