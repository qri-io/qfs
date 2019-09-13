package qfs

import (
	"bytes"
	"io/ioutil"
	"context"
	"testing"
)

func TestMemFS(t *testing.T) {
	ctx := context.Background()
	memfs := NewMemFS(testStore(0))
	f, err := memfs.Get(ctx, "path")
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
}

type testStore int

func (t testStore) Get(ctx context.Context, path string) (File, error) {
	if path == "path" {
		return NewMemfileBytes("path", []byte(`data`)), nil
	}

	return nil, ErrNotFound
}
