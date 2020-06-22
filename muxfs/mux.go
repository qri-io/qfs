package muxfs

import (
	"context"
	"fmt"
	"sync"

	"github.com/qri-io/qfs"
	"github.com/qri-io/qfs/cafs"
	"github.com/qri-io/qfs/httpfs"
	"github.com/qri-io/qfs/localfs"
	"github.com/qri-io/qfs/qipfs"
)

// Mux multiplexes together multiple filesystems using path multiplexing.
// It's a way to use multiple filesystem implementations as a single FS
type Mux struct {
	handlers map[string]qfs.Filesystem
	// sophisticated writes require a legacy cafs.Filesystem interface
	// the first configured filesystem that implements cafs.Filesystem
	// will be set to this string, and returned by the DefaultWriteFS method
	defaultWriteDestination string

	doneCh  chan struct{}
	doneWg  sync.WaitGroup
	doneErr error
}

// compile-time assertion that MapStore satisfies the Filesystem interface
var _ qfs.Filesystem = (*Mux)(nil)

// SetFilesystem designates the resolver for a given path kind string
func (m *Mux) SetFilesystem(fs qfs.Filesystem) error {
	if m.handlers == nil {
		m.handlers = map[string]qfs.Filesystem{}
	}

	if m.handlers[fs.Type()] != nil {
		return fmt.Errorf("mux already has a %q filesystem", fs.Type())
	}

	if releaser, ok := fs.(qfs.ReleasingFilesystem); ok {
		m.doneWg.Add(1)
		go func(releaser qfs.ReleasingFilesystem) {
			<-releaser.Done()
			m.doneErr = releaser.DoneErr()
			m.doneWg.Done()
		}(releaser)
	}
	if m.defaultWriteDestination == "" {
		if _, ok := fs.(cafs.Filestore); ok {
			m.defaultWriteDestination = fs.Type()
		}
	}

	m.handlers[fs.Type()] = fs
	return nil
}

// Filesystem returns the filesystem for a given fs type string, nil if no
// filesystem for fsType exists
func (m *Mux) Filesystem(fsType string) qfs.Filesystem {
	return m.handlers[fsType]
}

// constructors maps filesystem type strings to constructor functions
var constructors = map[string]qfs.Constructor{
	qipfs.FilestoreType:   qipfs.NewFilesystem,
	localfs.FilestoreType: localfs.NewFilesystem,
	httpfs.FilestoreType:  httpfs.NewFilesystem,
	qfs.MemFilestoreType:  qfs.NewMemFilesystem,
	cafs.MapFilestoreType: cafs.NewMapFilesystem,
}

// New creates a new Mux Filesystem, if no Option funcs are provided,
// New uses a default set of Option funcs. Any Option functions passed to this
// function must check whether their fields are nil or not.
// The first configured filesystem that implements the cafs.Filestore interface
// becomes the default filesystem returned by DefaultWriteFS
func New(ctx context.Context, cfgs []qfs.Config) (*Mux, error) {
	mux := &Mux{
		handlers: map[string]qfs.Filesystem{},
		doneCh:   make(chan struct{}),
	}
	for _, cfg := range cfgs {
		constructor, ok := constructors[cfg.Type]
		if !ok {
			return nil, fmt.Errorf("unrecognized filesystem type: %q", cfg.Type)
		}
		fs, err := constructor(ctx, cfg.Config)
		if err != nil {
			return nil, fmt.Errorf("constructing %q filesystem: %w", cfg.Type, err)
		}

		if err := mux.SetFilesystem(fs); err != nil {
			return nil, err
		}
	}

	go func() {
		mux.doneWg.Wait()
		close(mux.doneCh)
	}()

	return mux, nil
}

// FilestoreType uniquely identifies the mux filestore
const FilestoreType = "mux"

// Type distinguishes this filesystem from others by a unique string prefix
func (m *Mux) Type() string {
	return FilestoreType
}

// DoneErr will return any error value after the done channel is closed
func (m *Mux) DoneErr() error {
	return m.doneErr
}

// Done implements the qfs.ReleasingFilesystem interface
func (m *Mux) Done() <-chan struct{} {
	return m.doneCh
}

func noMuxerError(kind, path string) error {
	return fmt.Errorf("cannot resolve paths of kind '%s'. path: %s", kind, path)
}

// Get a path
func (m *Mux) Get(ctx context.Context, path string) (qfs.File, error) {
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
func (m *Mux) Put(ctx context.Context, file qfs.File) (resPath string, err error) {
	path := file.FullPath()
	kind := qfs.PathKind(path)
	handler, ok := m.handlers[kind]
	if !ok {
		return "", noMuxerError(kind, path)
	}

	return handler.Put(ctx, file)
}

// Delete removes a file or directory from the filesystem
func (m *Mux) Delete(ctx context.Context, path string) (err error) {
	kind := qfs.PathKind(path)
	handler, ok := m.handlers[kind]
	if !ok {
		return noMuxerError(kind, path)
	}

	return handler.Delete(ctx, path)
}

// DefaultWriteFS gives the muxer's configured write destination
// for legacy reasons, the returned destination must be a cafs.Filestore
func (m *Mux) DefaultWriteFS() cafs.Filestore {
	if m.defaultWriteDestination != "" {
		return m.handlers[m.defaultWriteDestination].(cafs.Filestore)
	}
	return nil
}
