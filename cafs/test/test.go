package test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/qri-io/qfs"
	"github.com/qri-io/qfs/cafs"
)

func EnsureFilestoreBehavior(f cafs.Filestore) error {
	if err := EnsureFilestoreSingleFileBehavior(f); err != nil {
		return err
	}
	if err := EnsureFilestoreAdderBehavior(f); err != nil {
		return err
	}
	return nil
}

func EnsureFilestoreSingleFileBehavior(f cafs.Filestore) error {
	ctx := context.Background()
	fdata := []byte("foo")
	file := qfs.NewMemfileBytes("file.txt", fdata)
	key, err := f.Put(ctx, file)
	if err != nil {
		return fmt.Errorf("Filestore.Put(%s) error: %s", file.FileName(), err.Error())
	}

	pre := "/" + f.Type() + "/"
	if !strings.HasPrefix(key, pre) {
		return fmt.Errorf("key returned didn't return a that matches this Filestore's Type. Expected: %s/..., got: %s", pre, key)
	}

	outf, err := f.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("Filestore.Get(%s) error: %s", key, err.Error())
	}
	data, err := ioutil.ReadAll(outf)
	if err != nil {
		return fmt.Errorf("error reading data from returned file: %s", err.Error())
	}
	if !bytes.Equal(fdata, data) {
		return fmt.Errorf("mismatched return value from get: %s != %s", string(fdata), string(data))
		// return fmt.Errorf("mismatched return value from get: %s != %s", outf.FileName(), string(data))
	}

	has, err := f.Has(ctx, "no-match")
	if err != nil {
		// error here shouldn't be a problem
		//  fmt.Errorf("Filestore.Has([nonexistent key]) error: %s", err.Error())
	}
	if has {
		return fmt.Errorf("filestore claims to have a very silly key")
	}

	has, err = f.Has(ctx, key)
	if err != nil {
		return fmt.Errorf("Filestore.Has(%s) error: %s", key, err.Error())
	}
	if !has {
		return fmt.Errorf("Filestore.Has(%s) should have returned true", key)
	}
	if err = f.Delete(ctx, key); err != nil {
		return fmt.Errorf("Filestore.Delete(%s) error: %s", key, err.Error())
	}

	return nil
}

func EnsureDirectoryBehavior(f cafs.Filestore) error {
	ctx := context.Background()

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
	qfs.Walk(outf, func(f qfs.File) error {
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
}

func EnsureFilestoreAdderBehavior(f cafs.Filestore) error {
	ctx := context.Background()

	adder, err := f.NewAdder(false, false)
	if err != nil {
		return fmt.Errorf("Filestore.NewAdder(false,false) error: %s", err.Error())
	}

	data := []byte("bar")
	if err := adder.AddFile(ctx, qfs.NewMemfileBytes("test.txt", data)); err != nil {
		return fmt.Errorf("Adder.AddFile error: %s", err.Error())
	}

	if err := adder.Close(); err != nil {
		return fmt.Errorf("Adder.Close() error: %s", err.Error())
	}

	return nil
}
