package localfs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/qri-io/qfs"
)

// FSConfig adjusts the behaviour of an FS instance
type FSConfig struct {
	PWD string // working directory. defaults to system root
}

// Option is a function type for passing to NewFS
type Option func(cfg *FSConfig)

// OptionSetPWD sets the present working directory for the FS
func OptionSetPWD(pwd string) Option {
	return func(cfg *FSConfig) {
		cfg.PWD = pwd
	}
}

// DefaultFSConfig is the configuration state with no additional options
// consumers of this package typically don't need to use this
func DefaultFSConfig() *FSConfig {
	return &FSConfig{
		PWD: "",
	}
}

// NewFS creates a new local filesytem PathResolver
func NewFS(opts ...Option) *FS {
	cfg := DefaultFSConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return &FS{cfg: cfg}
}

// FS is a implementation of qfs.PathResolver that uses the local filesystem
type FS struct {
	cfg *FSConfig
}

// compile-time assertion that MapStore satisfies the Filesystem interface
var _ qfs.Filesystem = (*FS)(nil)

// Get implements qfs.PathResolver
func (lfs *FS) Get(ctx context.Context, path string) (qfs.File, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, qfs.ErrNotFound
		}
		return nil, err
	}

	if fi.IsDir() {
		// TODO (b5): implement local directory support
		return nil, fmt.Errorf("local directory is not supported")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening local file: %s", err.Error())
	}

	return &LocalFile{
		path: path,
		File: *f,
	}, nil
}

// Put places a file or directory on the filesystem, returning the root path.
// The returned path may or may not honor the path of the given file
func (lfs *FS) Put(ctx context.Context, file qfs.File) (resultPath string, err error) {
	path := file.FullPath()
	// ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0666); err != nil {
		return "", err
	}

	if file.IsDirectory() {
		for {
			childFile, err := file.NextFile()
			if err != nil {
				if err.Error() == "EOF" {
					return path, err
				}

				return "", err
			}

			if _, err = lfs.Put(ctx, childFile); err != nil {
				return "", err
			}
		}
		return path, nil
	}

	f, err := os.Create(path)
	if err != nil {
		return path, err
	}
	defer f.Close()

	_, err = io.Copy(f, file)
	return path, err
}

// Delete removes a file or directory from the filesystem
func (lfs *FS) Delete(ctx context.Context, path string) (err error) {
	// TODO (b5):
	return fmt.Errorf("deleting local files via qfs.Localfs is not finished")
}

// LocalFile implements qfs.File with a filesystem file
type LocalFile struct {
	os.File
	path string
}

// IsDirectory satisfies the qfs.File interface
func (lf *LocalFile) IsDirectory() bool {
	return false
}

// NextFile satisfies the qfs.File interface
func (lf *LocalFile) NextFile() (qfs.File, error) {
	return nil, qfs.ErrNotDirectory
}

// FileName returns a filename associated with this file
func (lf *LocalFile) FileName() string {
	return filepath.Base(lf.path)
}

// FullPath returns the full path used when adding this file
func (lf *LocalFile) FullPath() string {
	return lf.path
}
