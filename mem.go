package qfs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/mr-tron/base58"
	"github.com/multiformats/go-multihash"
	"github.com/qri-io/qfs/fs"
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

// FSName distinguishes this filesystem from others by a unique string
func (m MemStore) FSName() string {
	return MemFilestoreType
}

// Open implements the fs.FS interface
func (m MemStore) Open(name string) (fs.File, error) {
	return m.OpenFile(context.Background(), name)
}

// Print converts the store to a string
func (m MemStore) Print() (string, error) {
	buf := &bytes.Buffer{}
	for key, file := range m.Files {
		data, err := ioutil.ReadAll(file.File())
		if err != nil {
			return "", err
		}
		if fi, err := file.File().Stat(); err == nil {
			fmt.Fprintf(buf, "%s:%s\n\t%s\n", key, fi.Name(), string(data))
		}
	}

	return buf.String(), nil
}

// WriteFile adds a file to the store
func (m *MemStore) WriteFile(ctx context.Context, file File) (string, error) {
	var name string
	fi, err := file.Stat()
	if err != nil {
		return "", err
	}

	switch fi.Mode() {
	case os.ModeDir:
		buf := &bytes.Buffer{}
		dir := fsDir{
			store: m,
			fi:    fi,
			files: []string{},
		}

		readDirFile, ok := file.(fs.ReadDirFile)
		if !ok {
			return "", fmt.Errorf("writing a directory requires the  ReadDirFile interface")
		}

		fileInfos, err := readDirFile.ReadDir(-1)
		if err != nil {
			return "", err
		}

		for _, fi := range fileInfos {
			f, err := m.OpenFile(ctx, fi.Name())
			if err != nil {
				return "", err
			}

			hash, err := m.WriteFile(ctx, f)
			if err != nil {
				return "", fmt.Errorf("putting file: %s", err.Error())
			}
			name = hash
			dir.files = append(dir.files, hash)
			if _, err = buf.WriteString(name + "\n"); err != nil {
				err = fmt.Errorf("error writing to buffer: %s", err.Error())
				return "", err
			}
		}

		dirhash, e := hashBytes(buf.Bytes())
		if err != nil {
			err = fmt.Errorf("error hashing file data: %s", e.Error())
			return "", err
		}

		name = NamePrefix(m) + dirhash
		m.Files[name] = dir
	case ModeLinkedData:
		data, err := ioutil.ReadAll(file)
		if err != nil {
			return "", fmt.Errorf("reading from file: %w", err)
		}
		hash, err := hashBytes(data)
		if err != nil {
			return "", fmt.Errorf("hashing file data: %w", err)
		}
		linkedData, err := file.LinkedData()
		if err != nil {
			return "", fmt.Errorf("getting linked data: %w", err)
		}
		links, err := GatherDereferencedLinksAsFileValues(linkedData)
		if err != nil {
			return "", err
		}
		for i, link := range links {
			name, err := m.WriteFile(ctx, link.Value.(File))
			if err != nil {
				return "", err
			}
			links[i].Ref = name
			links[i].Value = nil
		}
		name = NamePrefix(m) + hash
		m.Files[name] = fsFile{
			fi:    fi,
			data:  linkedData,
			bytes: data,
		}
	default:
		data, err := ioutil.ReadAll(file)
		if err != nil {
			return "", fmt.Errorf("reading from file: %w", err)
		}
		hash, err := hashBytes(data)
		if err != nil {
			return "", fmt.Errorf("hashing file data: %w", err)
		}
		name = NamePrefix(m) + hash
		m.Files[name] = fsFile{
			fi:    fi,
			bytes: data,
		}
	}

	return name, nil
}

// OpenFile returns a File from the store
func (m *MemStore) OpenFile(ctx context.Context, key string) (File, error) {
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
func (m MemStore) Delete(ctx context.Context, name string) error {
	if has, err := m.Has(ctx, name); err != nil {
		return err
	} else if !has {
		return ErrNotFound
	}

	delete(m.Files, name)
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
	fi    os.FileInfo
	data  interface{}
	bytes []byte
}

func (f fsFile) File() File {
	if f.fi.Mode() == ModeLinkedData {
		return linkedDataFile{
			fi:   f.fi,
			data: f.data,
			buf:  bytes.NewBuffer(f.bytes),
		}
	}

	file, err := NewFileWithInfo(f.fi, bytes.NewBuffer(f.bytes))
	if err != nil {
		panic(err)
	}
	return file
}

type fsDir struct {
	store *MemStore
	fi    os.FileInfo
	files []string
}

func (f fsDir) File() File {
	files := make([]File, len(f.files))
	for i, path := range f.files {
		file, err := f.store.OpenFile(context.TODO(), path)
		if err != nil {
			panic(path)
		}
		files[i] = file
	}

	return NewMemdir(f.fi.Name(), files...)
}

type filer interface {
	File() File
}

func collectDescendants(fileInfos []os.FileInfo, hashBuffer *bytes.Buffer, descendants *[]fsFile) (error) {
	for _, fi := range fileInfos {
		switch fi.Mode() {
		case os.ModeDir:
			collectDescendants(fileInfos []os.FileInfo, hashBuffer *bytes.Buffer, descendants *[]fsFile)
		case ModeLinkedData:
		default:
		}
		f, err := m.OpenFile(ctx, fi.Name())
		if err != nil {
			return err
		}

		hash, err := m.WriteFile(ctx, f)
		if err != nil {
			return fmt.Errorf("putting file: %s", err.Error())
		}
		name = hash
		dir.files = append(dir.files, hash)
		if _, err = buf.WriteString(name + "\n"); err != nil {
			err = fmt.Errorf("error writing to buffer: %s", err.Error())
			return  err
		}
	}
}
