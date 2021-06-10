package qfs

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMemfile(t *testing.T) {
	a := NewMemdir("/a",
		NewMemfileBytes("a.txt", []byte("foo")),
		NewMemfileBytes("b.txt", []byte("bar")),
		NewMemdir("/c",
			NewMemfileBytes("d.txt", []byte("baz")),
			NewMemdir("/e",
				NewMemfileBytes("f.txt", []byte("bat")),
			),
		),
		NewMemfileBytes("h.txt", []byte("bong")),
		NewMemfileBytes("/i/j.txt", []byte("boink")),
	)

	a.AddChildren(NewMemfileBytes("g.txt", []byte("kazam")))

	expectPaths := []string{
		"/a/a.txt",
		"/a/b.txt",
		"/a/c/d.txt",
		"/a/c/e/f.txt",
		"/a/c/e",
		"/a/c",
		"/a/h.txt",
		"/a/j.txt",
		"/a/g.txt",
		"/a",
	}

	paths := []string{}
	err := Walk(a, func(f File) error {
		paths = append(paths, f.FullPath())
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}
	if len(paths) != len(expectPaths) {
		t.Errorf("path length mismatch. expected: %d, got %d", len(expectPaths), len(paths))
		return
	}

	if diff := cmp.Diff(expectPaths, paths); diff != "" {
		t.Errorf("visited paths mismatch. (-want +got):\n%s", diff)
	}
}

func TestSizeFile(t *testing.T) {
	cases := []struct {
		file SizeFile
		size int64
	}{
		{NewMemfileBytes("", []byte{0, 1, 2, 3, 4, 5}), 6},
		{NewMemfileReader("", &bytes.Buffer{}), -1},
		{NewMemfileReaderSize("", &bytes.Buffer{}, 10), 10},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			if c.size != c.file.Size() {
				t.Errorf("size mismatch. want: %d got: %d ", c.size, c.file.Size())
			}
		})
	}
}

func TestMemdirMakeDirP(t *testing.T) {
	dir := NewMemdir("/")
	dir.MakeDirP(NewMemfileBytes("./a/b/c/d/file.txt", []byte("foo")))
	dir.MakeDirP(NewMemfileBytes("./a/b/file.txt", []byte("foo")))

	expectPaths := []string{
		// "/a/b/c/d/file.txt",
		"/a/b/c/d",
		"/a/b/c",
		"/a/b",
		"/a",
		"/",
	}

	paths := []string{}
	err := Walk(dir, func(f File) error {
		paths = append(paths, f.FullPath())
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}
	if len(paths) != len(expectPaths) {
		t.Errorf("path length mismatch. expected: %d, got %d", len(expectPaths), len(paths))
		return
	}

	for i, p := range expectPaths {
		if paths[i] != p {
			t.Errorf("path %d mismatch expected: %s, got: %s", i, p, paths[i])
		}
	}
}
