package cafs

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/qri-io/qfs"
)

func TestWriteHooks(t *testing.T) {
	root := qfs.NewMemdir("/a",
		qfs.NewMemfileBytes("b.txt", []byte("foo")),
		qfs.NewMemfileBytes("c.txt", []byte("bar")),
		qfs.NewMemfileBytes("d.txt", []byte("baz")),
	)

	ctx := context.Background()
	fs := NewMapstore()
	bHash := ""

	rewriteB := NewMerkelizeHook("/a/b.txt", func(ctx context.Context, f qfs.File, pathMap map[string]string) (io.Reader, error) {
		hContents, err := fs.Get(ctx, pathMap["/a/d.txt"])
		if err != nil {
			return nil, err
		}
		hData, err := ioutil.ReadAll(hContents)
		if err != nil {
			return nil, err
		}
		return strings.NewReader("APPLES" + string(hData)), nil
	}, "/a/d.txt")

	getBHash := NewMerkelizeHook("/a/c.txt", func(ctx context.Context, f qfs.File, pathMap map[string]string) (io.Reader, error) {
		bHash = pathMap["/a/b.txt"]
		return f, nil
	}, "/a/b.txt")

	_, err := WriteWithHooks(ctx, fs, root, rewriteB, getBHash)
	if err != nil {
		t.Fatal(err)
	}

	f, err := fs.Get(ctx, bHash)
	if err != nil {
		t.Fatalf("getting hooked file: %s", err)
	}
	gotData, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}

	expect := "APPLESbaz"
	if expect != string(gotData) {
		t.Errorf("stored result mismatch. want: %q got: %q", expect, string(gotData))
	}
}

func TestWriteHooksRollback(t *testing.T) {
	root := qfs.NewMemdir("/a",
		qfs.NewMemfileBytes("b.txt", []byte("foo")),
		qfs.NewMemfileBytes("c.txt", []byte("bar")),
		qfs.NewMemfileBytes("d.txt", []byte("baz")),
	)

	ctx := context.Background()
	fs := NewMapstore()
	errMsg := "oh noes it broke"

	rewriteB := NewMerkelizeHook("/a/b.txt", func(ctx context.Context, f qfs.File, pathMap map[string]string) (io.Reader, error) {
		return nil, fmt.Errorf(errMsg)
	}, "/a/d.txt")

	_, err := WriteWithHooks(ctx, fs, root, rewriteB)
	if err == nil {
		t.Errorf("expected error, got nil")
	} else if err.Error() != errMsg {
		t.Errorf("error mismatch. want: %q, got: %q", errMsg, err.Error())
	}

	expectCount := 0
	if count := fs.ObjectCount(); count != expectCount {
		t.Errorf("expected %d objects, got: %d", expectCount, count)
	}
}
