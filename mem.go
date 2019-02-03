package qfs

import "fmt"

// NewMemFS creates an in-memory filesystem from a set of files
func NewMemFS(files ...File) *MemFS {
	return &MemFS{}
}

// MemFS is an in-memory implementation
// It currently doesn't work, this is just a placeholder for upstream code
type MemFS struct {
}

// Get implements PathResolver interface
func (mfs *MemFS) Get(path string) (File, error) {
	return nil, fmt.Errorf("memfs is not yet finished")
}
