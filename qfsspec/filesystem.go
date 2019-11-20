package qfsspec

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/qri-io/qfs"
	"github.com/qri-io/value"
)

// RunFilesystemSpecTests executes the test suite against a given file sytem
func RunFilesystemSpecTests(t *testing.T, fs qfs.Filesystem) {
	requirements := []requirement{
		oneFile,
		oneFilePathPrefix,
		oneFileHas,
		// directories,
		// resolvedLinkPersistence
	}

	// TODO (b5) - put this on a deadline. It should be a prerequisite that this
	// test suite complete within a specified time span
	ctx := context.Background()

	for i, requirement := range requirements {
		t.Run(requirement.name, func(t *testing.T) {
			if err := requirement.fn(ctx, fs); err != nil {
				t.Errorf("requirement %d: %q failure:%s\ntest description:%s", i, requirement.name, err, requirement.description)
			}
		})
	}
}

type requirement struct {
	name        string
	description string
	fn          func(ctx context.Context, f qfs.Filesystem) error
}

var oneFile = requirement{
	name:        "OneFile",
	description: `Put, Get, Then Delete a single file`,
	fn: func(ctx context.Context, fs qfs.Filesystem) error {
		fdata := []byte("foo")
		file := qfs.NewMemfileBytes("file.txt", fdata)
		path, err := fs.Put(ctx, file)
		if err != nil {
			return fmt.Errorf("Filestore.Put(%s) error: %s", file.FileName(), err.Error())
		}

		outf, err := fs.Get(ctx, path)
		if err != nil {
			return fmt.Errorf("Filestore.Get(%s) error: %s", path, err.Error())
		}
		data, err := ioutil.ReadAll(outf)
		if err != nil {
			return fmt.Errorf("error reading data from returned file: %s", err.Error())
		}
		if !bytes.Equal(fdata, data) {
			return fmt.Errorf("mismatched return value from get: %s != %s", string(fdata), string(data))
		}

		if err = fs.Delete(ctx, path); err != nil {
			return fmt.Errorf("Filestore.Delete(%s) error: %s", path, err.Error())
		}

		return nil
	},
}

var oneFilePathPrefix = requirement{
	name:        "OneFilePathPrefix",
	description: "adding and removing a single file should return a path with a prefix from the set of fs.PathPrefixes()",
	fn: func(ctx context.Context, f qfs.Filesystem) error {
		file := qfs.NewMemfileBytes("requirement_prefix.txt", []byte("requirement_prefix"))
		path, err := f.Put(ctx, file)
		if err != nil {
			return fmt.Errorf("Filestore.Put(%s) error: %s", file.FileName(), err.Error())
		}

		prefixFound := false
		for _, prefix := range f.PathPrefixes() {
			if strings.HasPrefix(path, prefix) {
				prefixFound = true
				break
			}
		}

		if !prefixFound {
			return fmt.Errorf("adding a file didn't return a path with a prefix that matches PathPrefixes. path: %q, prefixes: %v", path, f.PathPrefixes())
		}

		if err = f.Delete(ctx, path); err != nil {
			return fmt.Errorf("Filestore.Delete(%s) error: %s", path, err.Error())
		}

		return nil
	},
}

var oneFileHas = requirement{
	name:        "OneFileHas",
	description: "Filesystem should report proper response for the .Has() before and after deletion",
	fn: func(ctx context.Context, fs qfs.Filesystem) error {
		badPath := "no-match"
		has, err := fs.Has(ctx, badPath)
		if err != nil {
			return fmt.Errorf("Filestore.Has(%q) error: %s", badPath, err.Error())
		}
		if has {
			return fmt.Errorf("filestore claims to have path %q, it shouldn't", badPath)
		}

		fdata := []byte("has requirement")
		file := qfs.NewMemfileBytes("has_requirement.txt", fdata)
		path, err := fs.Put(ctx, file)
		if err != nil {
			return fmt.Errorf("Filestore.Put(%s) error: %s", file.FileName(), err.Error())
		}

		has, err = fs.Has(ctx, path)
		if err != nil {
			return fmt.Errorf("Filestore.Has(%s) error: %s", path, err.Error())
		}
		if !has {
			return fmt.Errorf("Filestore.Has(%s) should have returned true", path)
		}
		if err = fs.Delete(ctx, path); err != nil {
			return fmt.Errorf("Filestore.Delete(%s) error: %s", path, err.Error())
		}
		return nil
	},
}

var directories = requirement{
	name:        "directories",
	description: "putting a hierarchy of directories",
	fn: func(ctx context.Context, f qfs.Filesystem) error {

		file := qfs.NewMemdir("/a",
			qfs.NewMemfileBytes("b.txt", []byte("a")),
			qfs.NewMemdir("c",
				qfs.NewMemfileBytes("d.txt", []byte("d")),
			),
			qfs.NewMemfileBytes("e.txt", []byte("e")),
		)
		key, err := f.Put(ctx, file)
		if err != nil {
			return fmt.Errorf("Filestore.Put(%s) error: %s", file.FileName(), err.Error())
		}

		outf, err := f.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("Filestore.Get(%s) error: %s", key, err.Error())
		}

		expectPaths := []string{
			"/a",
			"/a/b.txt",
			"/a/c",
			"/a/c/d.txt",
			"/a/e.txt",
		}

		paths := []string{}
		qfs.Walk(outf, 0, func(f qfs.File, depth int) error {
			paths = append(paths, f.FullPath())
			return nil
		})

		if len(paths) != len(expectPaths) {
			return fmt.Errorf("path length mismatch. expected: %d, got %d", len(expectPaths), len(paths))
		}

		for i, p := range expectPaths {
			if paths[i] != p {
				return fmt.Errorf("path %d mismatch expected: %s, got: %s", i, p, paths[i])
			}
		}

		if err = f.Delete(ctx, key); err != nil {
			return fmt.Errorf("Filestore.Delete(%s) error: %s", key, err.Error())
		}

		return nil
	},
}

var resolvedLinkPersistence = requirement{
	name:        "resolvedLinkPersistence",
	description: "supplying a resolved link must create a file and store that link",
	fn: func(ctx context.Context, fs qfs.Filesystem) error {
		l := value.NewResolvedLink("/link", "string value")
		f := qfs.NewMemfile("/link_example", l)
		_, err := fs.Put(ctx, f)
		if err != nil {
			return fmt.Errorf("f.Put() a link Memfile error: %q", err)
		}

		return nil
	},
}

// supplying a map containing resolved links must create
// supplying an unresolved link must result in a string
