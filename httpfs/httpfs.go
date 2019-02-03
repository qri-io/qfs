package httpfs

import (
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

// Get implements qfs.PathResolver
func (httpfs *FS) Get(path string) (qfs.File, error) {
	resp, err := httpfs.cfg.Client.Get(path)
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
