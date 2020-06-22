package muxfs

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/qri-io/qfs"
	"github.com/qri-io/qfs/qipfs"
)

func TestDefaultNewMux(t *testing.T) {
	path := filepath.Join(os.TempDir(), "muxfs_test")
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		t.Errorf("error creating temp dir: %s", err.Error())
		return
	}
	defer os.RemoveAll(path)

	if err := qipfs.InitRepo(path, ""); err != nil {
		t.Errorf("error intializing repo: %s", err.Error())
		return
	}

	ctx := context.Background()
	cfg := []qfs.Config{
		{Type: "ipfs", Config: map[string]interface{}{"path": path}},
		{Type: "http"},
		{Type: "local"},
		{Type: "mem"},
		{Type: "map"},
	}
	mfs, err := New(ctx, cfg)
	if err != nil {
		t.Errorf("error creating new mux: %s", err)
		return
	}
	for _, fsType := range []string{
		"ipfs",
		"http",
		"local",
		"mem",
		"map",
	} {
		if mfs.Filesystem(fsType) == nil {
			t.Errorf("expected filesystem for %q, got nil", fsType)
		}
	}
	if mfs.Filesystem("nonexistent") != nil {
		t.Errorf("expected nonexistent filesystem to return nil")
	}
}

func TestDefaultWriteFS(t *testing.T) {
	// create a mux that does NOT hav an ipfsFS
	mfs := &Mux{}
	ipfsFS := mfs.DefaultWriteFS()
	if ipfsFS != nil {
		t.Errorf("expected nil return on an empty mux fs")
	}

	// create a mux with an ipfsFS
	path := filepath.Join(os.TempDir(), "muxfs_test_cafs_from_ipfs")
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		t.Errorf("error creating temp dir: %s", err.Error())
		return
	}
	defer os.RemoveAll(path)

	if err := qipfs.InitRepo(path, ""); err != nil {
		t.Errorf("error intializing repo: %s", err.Error())
		return
	}

	ctx := context.Background()
	cfg := []qfs.Config{
		{Type: "ipfs", Config: map[string]interface{}{"path": path}},
	}
	mfs, err := New(ctx, cfg)
	if err != nil {
		t.Errorf("error creating new mux")
		return
	}
	ipfsFS = mfs.DefaultWriteFS()
	if ipfsFS == nil {
		t.Errorf("expected ipfsFS to exist, got nil")
		return
	}
}

func TestRepoLockPerContext(t *testing.T) {
	dir, _ := ioutil.TempDir("", "lock_drop_test")
	if err := qipfs.InitRepo(dir, ""); err != nil {
		t.Fatal(err)
	}

	cfg := []qfs.Config{
		{
			Type: "ipfs",
			Config: map[string]interface{}{
				"path": dir,
			},
		},
	}

	// create a context for the lifespan of the filesystem
	fsCtx, closeFsContext := context.WithCancel(context.Background())
	fsA, err := New(fsCtx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// use a request-scoped context for operations that use the filesystem
	reqCtx := context.Background()
	path, err := fsA.Put(reqCtx, qfs.NewMemfileBytes("/ipfs/hello.text", []byte(`oh hai there`)))
	if err != nil {
		t.Fatal(err)
	}

	// close the filesystem context. This must release the IPFS repo lock
	closeFsContext()
	<-fsA.Done()

	// TODO(b5) - I'd assume we also can't get, but this still seems to work
	// for some reason. Thankfully attempting to write to the datastore does in
	// fact fail
	// if _, err := fsA.Get(reqCtx, path); err == nil {
	// 	t.Errorf("expected and error opening file from closed context. got none")
	// }

	_, err = fsA.Put(reqCtx, qfs.NewMemfileBytes("/ipfs/hello.text", []byte(`oh hai there?`)))
	if err == nil {
		t.Errorf("expected and error putting file into closed store. got none")
	}

	// create a new context & filesystem
	fsCtx, closeFsContext = context.WithCancel(context.Background())
	defer closeFsContext()

	fsB, err := New(fsCtx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := fsB.Get(reqCtx, path); err != nil {
		t.Errorf("getting file: %s", err)
	}

}
