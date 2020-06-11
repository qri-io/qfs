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
	cfg := []MuxConfig{
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
	if _, err := GetResolver(mfs, "ipfs"); err != nil {
		t.Errorf(err.Error())
	}
	if _, err := GetResolver(mfs, "http"); err != nil {
		t.Errorf(err.Error())
	}
	if _, err := GetResolver(mfs, "local"); err != nil {
		t.Errorf(err.Error())
	}
	if _, err := GetResolver(mfs, "mem"); err != nil {
		t.Errorf(err.Error())
	}
	if _, err := GetResolver(mfs, "map"); err != nil {
		t.Errorf(err.Error())
	}
}

func TestOptSetIPFSPathWithConfig(t *testing.T) {
	// test empty muxConfig
	o := &[]MuxConfig{
		{
			Type:   "ipfs",
			Config: map[string]interface{}{"path": "bad/path"},
		},
	}
	path := "test/path"
	OptSetIPFSPath(path)(o)
	var ipfscfg MuxConfig

	if len(*o) != 1 {
		t.Errorf("expected MuxConfig slice to have length 1, got %d", len(*o))
		return
	}
	for _, mc := range *o {
		if mc.Type == "ipfs" {
			ipfscfg = mc
			break
		}
	}
	if ipfscfg.Type != "ipfs" {
		t.Errorf("expected MuxConfig of type 'ipfs' to exist, got %s", ipfscfg.Type)
		return
	}
	gotPath, ok := ipfscfg.Config["path"]
	if !ok {
		t.Errorf("expected ipfs map[string]interface config to have field 'fsRepoPath', but it does not")
		return
	}
	if gotPath != path {
		t.Errorf("expected fsRepoPath to be '%s', got '%s'", path, gotPath)
	}
}

func TestOptSetIPFSPathEmptyConfig(t *testing.T) {
	// nil should error
	var o *[]MuxConfig
	path := "test/path"
	if err := OptSetIPFSPath(path)(o); err == nil {
		t.Errorf("expected error when using nil MuxConfig, but didn't get one")
		return
	}

	// test empty muxConfig
	o = &[]MuxConfig{}
	if err := OptSetIPFSPath(path)(o); err != nil {
		t.Errorf("unexpected error when setting ipfs path: %s", err)
		return
	}

	var ipfscfg MuxConfig

	if len(*o) != 1 {
		t.Errorf("expected MuxConfig slice to have length 1, got %d", len(*o))
		return
	}
	for _, mc := range *o {
		if mc.Type == "ipfs" {
			ipfscfg = mc
			break
		}
	}
	if ipfscfg.Type != "ipfs" {
		t.Errorf("expected MuxConfig of type 'ipfs' to exist, got %s", ipfscfg.Type)
		return
	}
	gotPath, ok := ipfscfg.Config["path"]
	if !ok {
		t.Errorf("expected ipfs map[string]interface config to have field 'path', but it does not")
		return
	}
	if gotPath != path {
		t.Errorf("expected fsRepoPath to be '%s', got '%s'", path, gotPath)
	}
}

func TestCAFSFromIPFS(t *testing.T) {
	// create a mux that does NOT hav an ipfsFS
	mfs := &Mux{}
	ipfsFS := mfs.CAFSStoreFromIPFS()
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
	cfg := []MuxConfig{
		{Type: "ipfs", Config: map[string]interface{}{"path": path}},
	}
	mfs, err := New(ctx, cfg)
	if err != nil {
		t.Errorf("error creating new mux")
		return
	}
	ipfsFS = mfs.CAFSStoreFromIPFS()
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

	cfg := []MuxConfig{
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
