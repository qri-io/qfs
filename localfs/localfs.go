package localfs

import (
	"fmt"
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

// Get implements qfs.PathResolver
func (lfs *FS) Get(path string) (qfs.File, error) {
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
