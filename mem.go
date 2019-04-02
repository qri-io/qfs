package qfs

// MemFSStore is the minimum interface for creating a MemFS
type MemFSStore interface {
	Get(path string) (File, error)
}

// NewMemFS creates an in-memory filesystem from a set of files
func NewMemFS(store MemFSStore) *MemFS {
	return &MemFS{
		store: store,
	}
}

// MemFS is an in-memory implementation of the FileSystem interface. it's a
// minimal wrapper around anything that supports getting a file with a
// string key
type MemFS struct {
	store MemFSStore
}

// Get implements PathResolver interface
func (mfs *MemFS) Get(path string) (File, error) {
	return mfs.store.Get(path)
}
