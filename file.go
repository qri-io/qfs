package qfs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/qri-io/qfs/fs"
)

var (
	// NewTimestamp is the function qfs uses to generate timestamps, can be overridden
	// for tests
	NewTimestamp = func() time.Time { return time.Now() }
	// ErrNotDirectory is the result of attempting to perform "directory-like" operations on a file
	ErrNotDirectory = errors.New("This file is not a directory")
	// ErrNotFile is the result of attempting to perform "file like" operations on a directory
	ErrNotFile = errors.New("This is a directory")
	// ErrNotLinkedData is a canonical error for files that aren't hyperdata
	ErrNotLinkedData = errors.New("this is not a hyperdata document")
	// ErrNotFilenameSetter is the canonical error for trying to set a filename
	// when setting a filename isn't possible
	ErrNotFilenameSetter = errors.New("Cannot set filename")
)

// File is an interface that provides functionality for handling
// files/directories as values that can be supplied to commands. For
// directories, child files are accessed serially by calling `NextFile()`.
type File interface {
	fs.File

	IsLinkedData() bool
	LinkedData() (interface{}, error)
}

// FilenameSetter adds the capacity to modify the file name property
type FilenameSetter interface {
	SetFilename(path string) error
}

// // Walk traverses a file tree calling visit on each node
// func Walk(root File, depth int, visit func(f File, depth int) error) (err error) {
// 	if err := visit(root, depth); err != nil {
// 		return err
// 	}

// 	if st, err := root.Stat(); err == nil && st.IsDir() {
// 		for {
// 			f, err := root.NextFile()
// 			if err != nil {
// 				if err.Error() == "EOF" {
// 					break
// 				} else {
// 					return err
// 				}
// 			}

// 			if err := Walk(f, depth+1, visit); err != nil {
// 				return err
// 			}
// 		}
// 	}
// 	return nil
// }

type fileInfo struct {
	name    string      // base name of the file
	size    int64       // length in bytes for regular files; system-dependent for others
	mode    os.FileMode // file mode bits
	modTime time.Time   // modification time
	sys     interface{}
}

var _ os.FileInfo = (*fileInfo)(nil)

// NewFileInfo constructs an os.FileInfo from input data
func NewFileInfo(name string, size int64, mode os.FileMode, mtime time.Time, sys interface{}) os.FileInfo {
	return &fileInfo{
		name:    name,
		size:    size,
		mode:    mode,
		modTime: mtime,
		sys:     sys,
	}
}

func (fi fileInfo) Name() string       { return fi.name }
func (fi fileInfo) Size() int64        { return fi.size }
func (fi fileInfo) Mode() os.FileMode  { return fi.mode }
func (fi fileInfo) ModTime() time.Time { return fi.modTime }
func (fi fileInfo) IsDir() bool        { return fi.mode.IsDir() }
func (fi fileInfo) Sys() interface{}   { return fi.sys }

func (fi *fileInfo) SetFilename(name string) error {
	fi.name = name
	return nil
}

// memfile is an in-memory file
type memfile struct {
	fi  os.FileInfo
	buf io.Reader
}

// Confirm that memfile satisfies the File interface
var _ = (File)(&memfile{})

// NewFileWithInfo creates a new open file with provided file information
func NewFileWithInfo(fi os.FileInfo, r io.Reader) (File, error) {
	switch fi.Mode() {
	case ModeLinkedData:
		var linkedData interface{}

		data, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &linkedData); err != nil {
			return nil, err
		}

		return &linkedDataFile{
			fi:   fi,
			data: linkedData,
			buf:  bytes.NewBuffer(data),
		}, nil
	case os.ModeDir:
		return nil, fmt.Errorf("NewFileWithInfo doesn't support creating directories")
	default:
		return &memfile{
			fi:  fi,
			buf: r,
		}, nil
	}
}

// NewMemfileReader creates a file from an io.Reader
func NewMemfileReader(name string, r io.Reader) File {
	return &memfile{
		fi:  NewFileInfo(name, -1, 0, time.Now(), nil),
		buf: r,
	}
}

// NewMemfileBytes creates a file from a byte slice
func NewMemfileBytes(name string, data []byte) File {
	return &memfile{
		fi:  NewFileInfo(name, int64(len(data)), 0, time.Now(), nil),
		buf: bytes.NewBuffer(data),
	}
}

// Stat returns information for this file
func (m memfile) Stat() (os.FileInfo, error) {
	return m.fi, nil
}

// Read implements the io.Reader interface
func (m memfile) Read(p []byte) (int, error) {
	return m.buf.Read(p)
}

// Close closes the file, if the backing reader implements the io.Closer interface
// it will call close on the backing Reader
func (m memfile) Close() error {
	if closer, ok := m.buf.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (m memfile) IsLinkedData() bool {
	return false
}

func (m memfile) LinkedData() (interface{}, error) {
	return nil, ErrNotLinkedData
}

// MediaType for a memfile returns a mime type based on file extension
func (m memfile) MediaType() string {
	return mime.TypeByExtension(filepath.Ext(m.fi.Name()))
}

// SetFilename implements the FilenameSetter interface
func (m *memfile) SetFilename(name string) error {
	if ns, ok := m.fi.(FilenameSetter); ok {
		return ns.SetFilename(name)
	}

	return errors.New("cannot set filename")
}

// memdir is an in-memory directory
// Currently it only supports either memfile & memdir as links
type memdir struct {
	fi    os.FileInfo
	files []File
}

// Confirm that Memdir satisfies the File interface
var _ = (File)(&memdir{})

// NewMemdir creates a new Memdir, supplying zero or more links
func NewMemdir(name string, links ...File) File {
	d := &memdir{
		fi: NewFileInfo(name, -1, os.ModeDir, NewTimestamp(), nil),
	}
	// TODO (b5)
	// d.AddChildren(links...)
	return d
}

// Stat returns information for this file
func (d memdir) Stat() (os.FileInfo, error) {
	return d.fi, nil
}

// Read does nothing, exists so MemDir implements the File interface
func (memdir) Read([]byte) (int, error) {
	return 0, ErrNotFile
}

func (d *memdir) ReadDir(n int) ([]os.FileInfo, error) {
	var fis []os.FileInfo
	if n <= 0 {
		fis = make([]os.FileInfo, len(d.files))
	} else {
		fis = make([]os.FileInfo, n)
	}

	for i := range fis {
		if i == len(d.files) {
			break
		}
		fi, err := d.files[i].Stat()
		if err != nil {
			return nil, err
		}
		fis[i] = fi
	}

	return fis, nil
}

// Close does nothing, exists so MemDir implements the File interface
func (memdir) Close() error {
	return nil
}

func (d *memdir) IsLinkedData() bool { return false }

func (d *memdir) LinkedData() (interface{}, error) {
	return nil, ErrNotLinkedData
}

// SetFilename implements the FilnameSetter interface
func (d *memdir) SetFilename(name string) error {
	return errors.New("cannot set filename of an in-memory directory")
	// m.path = path
	// for _, f := range m.links {
	// 	if fps, ok := f.(FilnameSetter); ok {
	// 		fps.SetPath(filepath.Join(path, f.FileName()))
	// 	}
	// }
}

// // AddChildren allows any sort of file to be added, but only
// // implementations that implement the FilnameSetter interface will have
// // properly configured paths.
// func (d *memdir) AddChildren(fs ...File) {
// 	for _, f := range fs {
// 		if fps, ok := f.(FilenameSetter); ok {
// 			fps.SetFilename(filepath.Join(d.FullPath(), d.FileName()))
// 		}
// 		dir := m.MakeDirP(f)
// 		dir.links = append(dir.links, f)
// 	}
// }

// // ChildDir returns a child directory at dirname
// func (m *memdir) ChildDir(dirname string) File {
// 	if dirname == "" || dirname == "." || dirname == "/" {
// 		return m
// 	}
// 	for _, f := range m.links {
// 		if dir, ok := f.(*memdir); ok {
// 			if filepath.Base(dir.path) == dirname {
// 				return dir
// 			}
// 		}
// 	}
// 	return nil
// }

// // MakeDirP ensures all directories specified by the given file exist, returning
// // the deepest directory specified
// func (m *memdir) MakeDirP(f File) File {
// 	dirpath, _ := filepath.Split(f.FullPath())
// 	if dirpath == "" || dirpath == "/" {
// 		return m
// 	}
// 	dirs := strings.Split(dirpath[1:len(dirpath)-1], "/")
// 	if len(dirs) == 1 {
// 		return m
// 	}

// 	dir := m
// 	for _, dirname := range dirs {
// 		if ch := dir.ChildDir(dirname); ch != nil {
// 			dir = ch
// 			continue
// 		}
// 		ch := NewMemdir(filepath.Join(dir.FullPath(), dirname))
// 		dir.links = append(dir.links, ch)
// 		dir = ch
// 	}
// 	return dir
// }

// ModeLinkedData is a file mode that indicates a linked data file
const ModeLinkedData os.FileMode = os.ModeIrregular - 1

type linkedDataFile struct {
	fi   os.FileInfo
	data interface{}
	buf  io.Reader
}

// NewLinkedDataFile creates a file from a hyperdata object
func NewLinkedDataFile(name string, data interface{}) (File, error) {
	bufData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &linkedDataFile{
		fi:   NewFileInfo(name, -1, ModeLinkedData, NewTimestamp(), nil),
		data: data,
		buf:  bytes.NewBuffer(bufData),
	}, nil
}

// Stat returns information for this file
func (df linkedDataFile) Stat() (os.FileInfo, error) {
	return df.fi, nil
}

// Read implements the io.Reader interface
func (df linkedDataFile) Read(p []byte) (int, error) {
	return df.buf.Read(p)
}

// Close closes the file, if the backing reader implements the io.Closer interface
// it will call close on the backing Reader
func (df linkedDataFile) Close() error {
	if closer, ok := df.buf.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (df linkedDataFile) IsLinkedData() bool { return true }

func (df linkedDataFile) LinkedData() (interface{}, error) {
	return df.data, nil
}

// FileString is a utility function that consumes a file, returning a string of
// file byte contents. This is for debugging purposes only, and should never be
// used for-realsies, as it pulls the *entire* file into a byte slice
func FileString(f File) (File, string) {
	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Sprintf("error getting file stats: %q", err)
	}

	if fi.IsDir() {
		return f, fmt.Sprintf("directory: %s", fi.Name())
	}

	data, err := ioutil.ReadAll(f)
	if err != nil {
		data = []byte(fmt.Sprintf("reading file: %s", err.Error()))
	}

	f, err = NewFileWithInfo(fi, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Sprintf("error creating new file: %q", err)
	}
	return f, string(data)
}
