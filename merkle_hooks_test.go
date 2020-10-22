package qfs

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"
)

func TestWriteHooks(t *testing.T) {
	ctx := context.Background()
	fs := NewMemFS()
	bHash := ""

	rewriteB := func(ctx context.Context, f File, pathMap map[string]string) (io.Reader, error) {
		hContents, err := fs.Get(ctx, pathMap["/a/d.txt"])
		if err != nil {
			return nil, err
		}
		hData, err := ioutil.ReadAll(hContents)
		if err != nil {
			return nil, err
		}
		return strings.NewReader("APPLES" + string(hData)), nil
	}

	getBHash := func(ctx context.Context, f File, pathMap map[string]string) (io.Reader, error) {
		bHash = pathMap["/a/b.txt"]
		return f, nil
	}

	root := NewMemdir("/a",
		NewWriteHookFile(NewMemfileBytes("/a/b.txt", []byte("foo")), rewriteB, "/a/d.txt"),
		NewWriteHookFile(NewMemfileBytes("/a/c.txt", []byte("bar")), getBHash, "/a/b.txt"),
		NewMemfileBytes("d.txt", []byte("baz")),
	)

	_, err := WriteWithHooks(ctx, fs, root)
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
	ctx := context.Background()
	fs := NewMemFS()
	errMsg := "oh noes it broke"

	failHook := func(ctx context.Context, f File, pathMap map[string]string) (io.Reader, error) {
		return nil, fmt.Errorf(errMsg)
	}

	root := NewMemdir("/a",
		NewWriteHookFile(NewMemfileBytes("b.txt", []byte("foo")), failHook, "/a/d.txt"),
		NewMemfileBytes("c.txt", []byte("bar")),
		NewMemfileBytes("d.txt", []byte("baz")),
	)

	_, err := WriteWithHooks(ctx, fs, root)
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
