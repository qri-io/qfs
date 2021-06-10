package localfs

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/qri-io/qfs"
)

// FSConfig adjusts the behaviour of an FS instance
type FSConfig struct {
	PWD string // working directory. defaults to system root
}

// Option is a function type for passing to NewFS
type Option func(cfgMap *FSConfig)

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

// if no cfgMap is given, return the default config
func mapToConfig(cfgMap map[string]interface{}) (*FSConfig, error) {
	if cfgMap == nil {
		return DefaultFSConfig(), nil
	}
	cfg := &FSConfig{}
	if err := mapstructure.Decode(cfgMap, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// NewFilesystem creates a new local filesystem Pathresolver
// with no options
func NewFilesystem(_ context.Context, cfgMap map[string]interface{}) (qfs.Filesystem, error) {
	return NewFS(cfgMap)
}

// NewFS creates a new local filesytem PathResolver
func NewFS(cfgMap map[string]interface{}, opts ...Option) (qfs.Filesystem, error) {
	cfg, err := mapToConfig(cfgMap)
	if err != nil {
		return nil, err
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return &FS{cfg: cfg}, nil
}

// FS is a implementation of qfs.PathResolver that uses the local filesystem
type FS struct {
	cfg *FSConfig
}

// compile-time assertion that MapStore satisfies the Filesystem interface
var _ qfs.Filesystem = (*FS)(nil)

// FilestoreType uniquely identifies this filestore
const FilestoreType = "local"

// Type distinguishes this filesystem from others by a unique string prefix
func (lfs *FS) Type() string {
	return FilestoreType
}

// Has returns whether the store has a File with the key
func (lfs *FS) Has(ctx context.Context, path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

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
		File: *f,
		info: fi,
		path: path,
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
	info os.FileInfo
	path string
}

var (
	_ qfs.File     = (*LocalFile)(nil)
	_ qfs.SizeFile = (*LocalFile)(nil)
)

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

// MediaType returns a mime type based on file extension
func (lf *LocalFile) MediaType() string {
	return mime.TypeByExtension(filepath.Ext(lf.path))
}

// ModTime returns time of last modification, if any
func (lf *LocalFile) ModTime() time.Time {
	st, err := os.Stat(lf.path)
	if err != nil {
		return time.Time{}
	}
	return st.ModTime()
}

func (lf *LocalFile) Size() int64 {
	return lf.info.Size()
}
