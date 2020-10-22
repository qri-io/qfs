package qfs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/mr-tron/base58"
	"github.com/multiformats/go-multihash"
)

// NewMemFilesystem allocates an instace of a mapstore that
// can be used as a PathResolver
// satisfies the FSConstructor interface
func NewMemFilesystem(_ context.Context, cfg map[string]interface{}) (Filesystem, error) {
	return NewMemFS(), nil
}

// NewMemFS allocates an instance of a mapstore
func NewMemFS() *MemStore {
	return &MemStore{
		Files: make(map[string]filer),
	}
}

// MemStore implements Filestore in-memory as a map
//
// An example pulled from tests will create a tree of "cafs"
// with directories & cafs, with paths properly set:
// NewMemdir("/a",
// 	NewMemfileBytes("a.txt", []byte("foo")),
// 	NewMemfileBytes("b.txt", []byte("bar")),
// 	NewMemdir("/c",
// 		NewMemfileBytes("d.txt", []byte("baz")),
// 		NewMemdir("/e",
// 			NewMemfileBytes("f.txt", []byte("bat")),
// 		),
// 	),
// )
// File is an interface that provides functionality for handling
// cafs/directories as values that can be supplied to commands.
type MemStore struct {
	Pinned  bool
	Network []*MemStore
	Files   map[string]filer
}

// compile-time assertion that MemStore satisfies the Filesystem interface
var _ Filesystem = (*MemStore)(nil)

// MemFilestoreType uniquely identifies the mem filestore
const MemFilestoreType = "mem"

// Type distinguishes this filesystem from others by a unique string prefix
func (m MemStore) Type() string {
	return MemFilestoreType
}

// Print converts the store to a string
func (m MemStore) Print() (string, error) {
	buf := &bytes.Buffer{}
	for key, file := range m.Files {
		f := file.File()
		if !f.IsDirectory() {
			data, err := ioutil.ReadAll(f)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(buf, "%s:%s\n\t%s\n", key, f.FullPath(), string(data))
		}
	}

	return buf.String(), nil
}

// ObjectCount returns the number of content-addressed objects in the store
func (m MemStore) ObjectCount() (objects int) {
	for range m.Files {
		objects++
	}
	return objects
}

// Put adds a file to the store
func (m *MemStore) Put(ctx context.Context, file File) (key string, err error) {
	if file.IsDirectory() {
		buf := bytes.NewBuffer(nil)
		dir := fsDir{
			store: m,
			path:  file.FullPath(),
			files: []string{},
		}

		for {
			f, e := file.NextFile()
			if e != nil {
				if e.Error() == "EOF" {
					dirhash, e := hashBytes(buf.Bytes())
					if err != nil {
						err = fmt.Errorf("error hashing file data: %s", e.Error())
						return
					}

					key = "/map/" + dirhash
					m.Files[key] = dir
					return
				}
				err = fmt.Errorf("error getting next file: %s", err.Error())
				return
			}

			hash, e := m.Put(ctx, f)
			if e != nil {
				err = fmt.Errorf("error putting file: %s", e.Error())
				return
			}
			key = hash
			dir.files = append(dir.files, hash)
			_, err = buf.WriteString(key + "\n")
			if err != nil {
				err = fmt.Errorf("error writing to buffer: %s", err.Error())
				return
			}
		}

	} else {
		data, e := ioutil.ReadAll(file)
		if e != nil {
			err = fmt.Errorf("error reading from file: %s", e.Error())
			return
		}
		hash, e := hashBytes(data)
		if e != nil {
			err = fmt.Errorf("error hashing file data: %s", e.Error())
			return
		}
		key = "/map/" + hash
		m.Files[key] = fsFile{name: file.FileName(), path: file.FullPath(), data: data}
		return
	}
}

// Get returns a File from the store
func (m *MemStore) Get(ctx context.Context, key string) (File, error) {
	// key may be of the form /map/QmFoo/file.json but MemStore indexes its maps
	// using keys like /map/QmFoo. Trim after the second part of the key.
	parts := strings.Split(key, "/")
	if len(parts) > 2 {
		prefix := strings.Join([]string{"", parts[1], parts[2]}, "/")
		key = prefix
	}
	// Check if the local MemStore has the file.
	f, err := m.getLocal(key)
	if err == nil {
		return f, nil
	} else if err != ErrNotFound {
		return nil, err
	}

	return nil, ErrNotFound
}

func (m *MemStore) getLocal(key string) (File, error) {
	if m.Files[key] == nil {
		return nil, ErrNotFound
	}
	return m.Files[key].File(), nil
}

// Has returns whether the store has a File with the key
func (m MemStore) Has(ctx context.Context, key string) (exists bool, err error) {
	if m.Files[key] == nil {
		return false, nil
	}
	return true, nil
}

// Delete removes the file from the store with the key
func (m MemStore) Delete(ctx context.Context, key string) error {
	delete(m.Files, key)
	return nil
}

// NewAdder returns an Adder for the store
func (m MemStore) NewAdder(ctx context.Context, pin, wrap bool) (Adder, error) {
	addedOut := make(chan AddedFile, 9)
	return &adder{
		fs:  m,
		out: addedOut,
	}, nil
}

type adder struct {
	fs  MemStore
	pin bool
	out chan AddedFile
}

func (a *adder) AddFile(ctx context.Context, f File) error {
	path, err := a.fs.Put(ctx, f)
	if err != nil {
		err = fmt.Errorf("error putting file in mapstore: %s", err.Error())
		return err
	}
	a.out <- AddedFile{
		Path:  path,
		Name:  f.FullPath(),
		Bytes: 0,
		Hash:  path,
	}
	return nil
}

func (a *adder) Added() chan AddedFile {
	return a.out
}

func (a *adder) Close() error {
	close(a.out)
	return nil
}

func hashBytes(data []byte) (hash string, err error) {
	h := sha256.New()
	if _, err = h.Write(data); err != nil {
		err = fmt.Errorf("error writing hash data: %s", err.Error())
		return
	}
	mhBuf, err := multihash.Encode(h.Sum(nil), multihash.SHA2_256)
	if err != nil {
		err = fmt.Errorf("error encoding hash: %s", err.Error())
		return
	}
	hash = base58.Encode(mhBuf)
	return
}

type fsFile struct {
	name string
	path string
	data []byte
}

func (f fsFile) File() File {
	return NewMemfileBytes(f.path, f.data)
}

type fsDir struct {
	store *MemStore
	path  string
	files []string
}

func (f fsDir) File() File {
	files := make([]File, len(f.files))
	for i, path := range f.files {
		file, err := f.store.Get(context.TODO(), path)
		if err != nil {
			panic(path)
		}
		files[i] = file
	}

	return NewMemdir(f.path, files...)
}

type filer interface {
	File() File
}
