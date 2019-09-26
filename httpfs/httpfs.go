package httpfs

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/qri-io/qfs"
)

// FSConfig adjusts the behaviour of an FS instance
type FSConfig struct {
	Client *http.Client // client to use to make requests
}

// Option is a function type for passing to NewFS
type Option func(cfg *FSConfig)

// OptionSetHTTPClient sets the http client to use
func OptionSetHTTPClient(cli *http.Client) Option {
	return func(cfg *FSConfig) {
		cfg.Client = cli
	}
}

// DefaultFSConfig is the configuration state with no additional options
// consumers of this package typically don't need to use this
func DefaultFSConfig() *FSConfig {
	return &FSConfig{
		Client: http.DefaultClient,
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

// compile-time assertino that MapStore satsfies the Filesystem interface
var _ qfs.Filesystem = (*FS)(nil)

// Get implements qfs.PathResolver
func (httpfs *FS) Get(ctx context.Context, path string) (qfs.File, error) {
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := httpfs.cfg.Client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, qfs.ErrNotFound
	}

	return &HTTPResFile{
		path: path,
		res:  resp,
	}, nil
}

// Put places a file or directory on the filesystem, returning the root path.
// The returned path may or may not honor the path of the given file
func (httpfs *FS) Put(ctx context.Context, file qfs.File) (resultPath string, err error) {
	return "", qfs.ErrReadOnly
}

// Delete removes a file or directory from the filesystem
func (httpfs *FS) Delete(ctx context.Context, path string) (err error) {
	return qfs.ErrReadOnly
}

// HTTPResFile implements qfs.File with a filesystem file
type HTTPResFile struct {
	res  *http.Response
	path string
}

// Read proxies to the response body reader
func (rf *HTTPResFile) Read(p []byte) (int, error) {
	return rf.res.Body.Read(p)
}

// Close proxies to the response body reader
func (rf *HTTPResFile) Close() error {
	return rf.res.Body.Close()
}

// IsDirectory satisfies the qfs.File interface
func (rf *HTTPResFile) IsDirectory() bool {
	return false
}

// NextFile satisfies the qfs.File interface
func (rf *HTTPResFile) NextFile() (qfs.File, error) {
	return nil, qfs.ErrNotDirectory
}

// FileName returns a filename associated with this file
func (rf *HTTPResFile) FileName() string {
	return filepath.Base(rf.path)
}

// FullPath returns the full path used when adding this file
func (rf *HTTPResFile) FullPath() string {
	return rf.path
}
