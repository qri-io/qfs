package qfs

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestMemFS(t *testing.T) {
	memfs := NewMemFS(testStore(0))
	f, err := memfs.Get("path")
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

func (t testStore) Get(path string) (File, error) {
	if path == "path" {
		return NewMemfileBytes("path", []byte(`data`)), nil
	}

	return nil, ErrNotFound
}
