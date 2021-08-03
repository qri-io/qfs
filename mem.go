package qfs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"sort"
	"strings"
	"sync"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	merkledag "github.com/ipfs/go-merkledag"
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
func NewMemFS() *MemFS {
	return &MemFS{
		Files: make(map[string]filer),
	}
}

// MemFS implements Filestore in-memory as a map
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
type MemFS struct {
	Pinned  bool
	Network []*MemFS

	filesLk sync.Mutex
	Files   map[string]filer
}

// compile-time assertions
var (
	_ Filesystem     = (*MemFS)(nil)
	_ CAFS           = (*MemFS)(nil)
	_ MerkleDagStore = (*MemFS)(nil)
)

// MemFilestoreType uniquely identifies the mem filestore
const MemFilestoreType = "mem"

// Type distinguishes this filesystem from others by a unique string prefix
func (m *MemFS) Type() string {
	return MemFilestoreType
}

func (m *MemFS) IsContentAddressedFilesystem() {}

// Print converts the store to a string
func (m *MemFS) Print() (string, error) {
	m.filesLk.Lock()
	defer m.filesLk.Unlock()

	buf := &bytes.Buffer{}
	for key, file := range m.Files {
		f, err := file.File()
		if err != nil {
			return "", err
		}
		if !f.IsDirectory() {
			data, err := ioutil.ReadAll(f)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(buf, "%s%s\n\t%s\n", key, f.FullPath(), string(data))
		} else {
			fmt.Fprintf(buf, "%s%s\n\tDIR:%#v\n", key, f.FullPath(), file.(fsDir).files)
		}
	}

	return buf.String(), nil
}

// ObjectCount returns the number of content-addressed objects in the store
func (m *MemFS) ObjectCount() (objects int) {
	return len(m.Files)
}

// PutFileAtKey puts the file at the given key
// Deprecated - this method breaks CAFS interface assertions. Don't use it.
func (m *MemFS) PutFileAtKey(ctx context.Context, key string, file File) error {
	if file.IsDirectory() {
		return fmt.Errorf("PutFileAtKey does not work with directories")
	}
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	m.Files[key] = fsFile{name: file.FileName(), path: file.FullPath(), data: data}
	return nil
}

// Put adds a file to the store
func (m *MemFS) Put(ctx context.Context, file File) (key string, err error) {
	key, err = m.put(ctx, file)
	return fmt.Sprintf("/%s/%s", MemFilestoreType, key), err
}

func (m *MemFS) put(ctx context.Context, file File) (key string, err error) {

	if file.IsDirectory() {
		buf := bytes.NewBuffer(nil)
		dir := fsDir{
			fs:    m,
			path:  file.FullPath(),
			files: map[string]string{},
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

					key = dirhash
					m.filesLk.Lock()
					m.Files[dirhash] = dir
					m.filesLk.Unlock()
					return
				}
				err = fmt.Errorf("error getting next file: %s", err.Error())
				return
			}

			hash, e := m.put(ctx, f)
			if e != nil {
				err = fmt.Errorf("error putting file: %s", e.Error())
				return
			}
			key = hash
			m.filesLk.Lock()
			dir.files[f.FileName()] = hash
			m.filesLk.Unlock()
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
		m.filesLk.Lock()
		m.Files[hash] = fsFile{name: file.FileName(), path: file.FullPath(), data: data}
		m.filesLk.Unlock()
		key = hash
		return
	}
}

// Get returns a File from the store
func (m *MemFS) Get(ctx context.Context, key string) (File, error) {
	// Check if the local MapStore has the file.
	f, err := m.getLocal(key)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Check if the anyone connected on the mock Network has the file.
			for _, connect := range m.Network {
				f, err := connect.getLocal(key)
				if err == nil {
					return f, nil
				} else if err != ErrNotFound {
					return nil, err
				}
			}
			return nil, ErrNotFound
		}
		return nil, err
	}

	return f, nil
}

func (m *MemFS) getLocal(key string) (File, error) {
	key = strings.TrimPrefix(key, fmt.Sprintf("/%s/", MemFilestoreType))
	// key may be of the form /mem/QmFoo/file.json but MemFS indexes its maps
	// using keys like /mem/QmFoo. Trim after the second part of the key.
	parts := strings.Split(key, "/")
	log.Debugw("MemFS getting", "key", key, "parts", parts)

	if len(parts) == 0 {
		return nil, fmt.Errorf("key is required")
	}

	m.filesLk.Lock()
	defer m.filesLk.Unlock()

	log.Debugw("get", "hash", parts[0])
	// Check if the local MemFS has the file
	f := m.Files[parts[0]]
	if f == nil {
		return nil, ErrNotFound
	}

	parts = parts[1:]
	for len(parts) > 0 {
		dir, ok := f.(fsDir)
		if !ok {
			return nil, ErrNotDirectory
		}
		log.Debugf("get part=%s files=%v", parts[0], dir.files)
		f = m.Files[dir.files[parts[0]]]
		if f == nil {
			return nil, ErrNotFound
		}
		parts = parts[1:]
	}

	return f.File()
}

// Has returns whether the store has a File with the key
func (m *MemFS) Has(ctx context.Context, key string) (exists bool, err error) {
	if _, err := m.getLocal(key); err == nil {
		return true, nil
	}
	return false, nil
}

// Delete removes the file from the store with the key
func (m *MemFS) Delete(ctx context.Context, key string) error {

	key = strings.TrimPrefix(key, fmt.Sprintf("/%s/", MemFilestoreType))
	// key may be of the form /mem/QmFoo/file.json but MemFS indexes its maps
	// using keys like /mem/QmFoo. Trim after the second part of the key.
	parts := strings.Split(key, "/")
	log.Debugf("MemFS deleting key=%q parts=%q", key, parts)

	if len(parts) == 0 {
		return fmt.Errorf("path is required")
	} else if len(parts) > 1 {
		return fmt.Errorf("can only delete entire hash, not individual paths")
	}

	// TODO (b5)
	log.Debugf("deleting root hash=%q", parts[0])
	m.filesLk.Lock()
	delete(m.Files, parts[0])
	m.filesLk.Unlock()
	return nil
	// return m.walkRm(parts[0])
}

func (m *MemFS) GetNode(id cid.Cid, path ...string) (DagNode, error) {
	m.filesLk.Lock()
	defer m.filesLk.Unlock()

	log.Debugw("get node", "cid", id.String(), "files", m.Files)
	f, ok := m.Files[id.String()]
	if !ok {
		return nil, ErrNotFound
	}

	file, err := f.File()
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return &memDagNode{
		id:   id,
		size: int64(len(data)),
		node: merkledag.NodeWithData(data),
	}, nil
}

func (m *MemFS) PutNode(links Links) (PutResult, error) {
	buf := bytes.NewBuffer(nil)
	dir := fsDir{
		fs:    m,
		path:  "",
		files: map[string]string{},
	}

	for _, ch := range links.SortedSlice() {
		dir.files[ch.Name] = ch.Cid.String()
		if _, err := buf.WriteString(ch.Cid.String() + "\n"); err != nil {
			panic(err.Error())
		}
	}

	mh, err := multihash.Sum(buf.Bytes(), multihash.SHA2_256, -1)
	if err != nil {
		return PutResult{}, err
	}

	id := cid.NewCidV0(mh)
	m.Files[id.String()] = dir
	return PutResult{
		Cid:  id,
		Size: int64(buf.Len()),
	}, nil
}

func (m *MemFS) GetBlock(id cid.Cid) (io.Reader, error) {
	m.filesLk.Lock()
	defer m.filesLk.Unlock()
	filer, ok := m.Files[id.String()]
	if !ok {
		return nil, ErrNotFound
	}

	return filer.File()
}

func (m *MemFS) PutBlock(d []byte) (id cid.Cid, err error) {
	res, err := m.putBlock("", d)
	return res.Cid, nil
}

func (m *MemFS) putBlock(name string, data []byte) (PutResult, error) {
	m.filesLk.Lock()
	defer m.filesLk.Unlock()

	hash, err := multihash.Sum(data, multihash.SHA2_256, -1)
	if err != nil {
		return PutResult{}, err
	}

	id := cid.NewCidV0(hash)
	m.Files[id.String()] = fsFile{
		name: name,
		path: "",
		data: data,
	}

	return PutResult{
		Cid:  id,
		Size: int64(len(data)),
	}, nil
}

func (m *MemFS) PutFile(f fs.File) (PutResult, error) {
	stat, err := f.Stat()
	if err != nil {
		return PutResult{}, err
	}

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return PutResult{}, err
	}
	if err := f.Close(); err != nil {
		return PutResult{}, err
	}

	return m.putBlock(stat.Name(), data)
}

func (m *MemFS) GetFile(root cid.Cid, path ...string) (io.ReadCloser, error) {
	if len(path) > 0 {
		return nil, fmt.Errorf("memfs does not support pathing beyond a root CID")
	}

	m.filesLk.Lock()
	defer m.filesLk.Unlock()

	f, ok := m.Files[root.String()]
	if !ok {
		return nil, ErrNotFound
	}

	return f.File()
}

type memDagNode struct {
	id   cid.Cid
	size int64
	node format.Node
}

var _ DagNode = (*memDagNode)(nil)

func (n memDagNode) Size() int64  { return n.size }
func (n memDagNode) Cid() cid.Cid { return n.id }
func (n memDagNode) Raw() []byte  { return n.node.RawData() }
func (n memDagNode) Links() Links {
	links := NewLinks()
	for _, link := range n.node.Links() {
		links.Add(Link{
			Name: link.Name,
			Cid:  link.Cid,
			Size: int64(link.Size),
		})
	}
	return links
}

func (m *MemFS) walkRm(hash string) error {
	f := m.Files[hash]
	if f == nil {
		return ErrNotFound
	}

	dir, ok := f.(fsDir)
	if !ok {
		delete(m.Files, hash)
		return nil
	}

	for _, chHash := range dir.files {
		if err := m.walkRm(chHash); err != nil {
			return err
		}
	}
	delete(m.Files, hash)
	return nil
}

// AddConnection sets up pointers from this MapStore to that, and vice versa.
func (m *MemFS) AddConnection(other *MemFS) {
	if other == m {
		return
	}
	// Add pointer from that network to this one.
	found := false
	for _, elem := range m.Network {
		if other == elem {
			found = true
		}
	}
	if !found {
		m.Network = append(m.Network, other)
	}
	// Add pointer from this network to that one.
	found = false
	for _, elem := range other.Network {
		if m == elem {
			found = true
		}
	}
	if !found {
		other.Network = append(other.Network, m)
	}
}

type adder struct {
	fs   *MemFS
	pin  bool
	out  chan AddedFile
	root string
	tree *nd
}

// NewAdder returns an Adder for the store
func (m *MemFS) NewAdder(ctx context.Context, pin, wrap bool) (Adder, error) {
	addedOut := make(chan AddedFile, 9)
	return &adder{
		fs:   m,
		out:  addedOut,
		tree: newNode(""),
	}, nil
}

func (a *adder) addNode(f File) *nd {
	path := f.FullPath()
	path = strings.TrimPrefix(path, fmt.Sprintf("/%s/", MemFilestoreType))
	path = strings.TrimPrefix(path, "/")

	node := a.tree
	if path == "" {
		return node
	}

	for _, part := range strings.Split(path, "/") {
		var ch *nd
		for _, c := range node.children {
			if c.name == part {
				ch = c
				break
			}
		}
		if ch == nil {
			ch = newNode(part)
			node.children = append(node.children, ch)
		}
		node = ch
	}
	return node
}

func (a *adder) AddFile(ctx context.Context, f File) (err error) {
	log.Debugf("Adder.AddFile FullPath=%s", f.FullPath())

	node := a.addNode(f)
	var hash string

	if f.IsDirectory() {
		var dir fsDir
		hash, dir = node.toDir(a.fs)
		if err != nil {
			return err
		}
		log.Debugf("adding directory path=%s dir=%v", hash, dir.files)
		a.fs.filesLk.Lock()
		a.fs.Files[hash] = dir
		a.fs.filesLk.Unlock()
		node.hash = hash
	} else {
		hash, err = a.fs.put(ctx, f)
		if err != nil {
			err = fmt.Errorf("error putting file in mapstore: %s", err.Error())
			return err
		}
		node.hash = hash
	}

	hash = fmt.Sprintf("/%s/%s", MemFilestoreType, hash)
	log.Debugf("Adder AddedFile FullPath=%s hash=%s", f.FullPath(), hash)
	a.root = hash
	a.out <- AddedFile{
		Path:  hash,
		Name:  f.FullPath(),
		Bytes: 0,
		Hash:  hash,
	}
	return nil
}

func (a *adder) Added() chan AddedFile {
	return a.out
}

func (a *adder) Finalize() (string, error) {
	close(a.out)

	log.Debugf("adding root directory")
	root := NewMemdir("/")
	node := a.addNode(root)
	hash, dir := node.toDir(a.fs)
	a.fs.filesLk.Lock()
	a.fs.Files[hash] = dir
	a.fs.filesLk.Unlock()

	node.hash = hash

	hash = fmt.Sprintf("/%s/%s", MemFilestoreType, hash)
	return hash, nil
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

func (f fsFile) File() (File, error) {
	return NewMemfileBytes(f.path, f.data), nil
}

type fsDir struct {
	fs    *MemFS
	path  string
	files map[string]string
}

func (f fsDir) File() (File, error) {
	files := make([]File, 0, len(f.files))

	for fileName, hash := range f.files {
		f := f.fs.Files[hash]
		if f == nil {
			return nil, fmt.Errorf("%w: fileName: %s hash: %s", ErrNotFound, fileName, hash)
		}
		file, err := f.File()
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}

	return NewMemdir(f.path, files...), nil
}

type filer interface {
	File() (File, error)
}

type nd struct {
	name     string
	hash     string
	children nodes
}

type nodes []*nd

func (ns nodes) Len() int           { return len(ns) }
func (ns nodes) Less(i, j int) bool { return ns[i].name < ns[j].name }
func (ns nodes) Swap(i, j int)      { ns[j], ns[i] = ns[i], ns[j] }

func newNode(name string) *nd {
	return &nd{name: name}
}

func (n *nd) toDir(fs *MemFS) (string, fsDir) {
	buf := bytes.NewBuffer(nil)
	dir := fsDir{
		fs:    fs,
		path:  n.name,
		files: map[string]string{},
	}

	sort.Sort(n.children)
	for _, ch := range n.children {
		dir.files[ch.name] = ch.hash
		if _, err := buf.WriteString(ch.hash + "\n"); err != nil {
			panic(err.Error())
		}
	}

	hash, err := hashBytes(buf.Bytes())
	if err != nil {
		panic(err)
	}
	return hash, dir
}
