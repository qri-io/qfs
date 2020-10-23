package qfs

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"testing"
)

func TestMemFS(t *testing.T) {
	ctx := context.Background()
	fs := NewMemFS()

	hash, err := fs.Put(ctx, NewMemfileBytes("path", []byte(`data`)))
	if err != nil {
		t.Fatal(err)
	}

	f, err := fs.Get(ctx, hash)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, []byte(`data`)) {
		t.Errorf("byte mismatch. expected: %s. got: %s", `data`, string(data))
	}

	dir := NewMemdir("/",
		NewMemdir("b",
			NewMemfileBytes("a.txt", []byte(`this is file a`)),
		),
	)

	dirHash, err := fs.Put(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}

	f, err = fs.Get(ctx, fmt.Sprintf("%s/b/a.txt", dirHash))
	if err != nil {
		t.Fatal(err)
	}

	data, err = ioutil.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, []byte(`this is file a`)) {
		t.Errorf("byte mismatch. expected: %s. got: %s", `this is file a`, string(data))
	}
}

type testStore int

func (t testStore) Get(ctx context.Context, path string) (File, error) {
	if path == "path" {
		return NewMemfileBytes("path", []byte(`data`)), nil
	}

	return nil, ErrNotFound
}
