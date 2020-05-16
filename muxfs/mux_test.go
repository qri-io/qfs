package muxfs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	qipfs "github.com/qri-io/qfs/cafs/ipfs"
)

func TestDefaultNewMux(t *testing.T) {
	// create a mux that has an fs for
	// ipfs
	// httpfs
	// localfs
	// mem

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
		{Type: "ipfs", Config: map[string]interface{}{"fsRepoPath": path}},
		{Type: "httpfs"},
		{Type: "localfs"},
		{Type: "mem"},
	}
	mfs, err := New(ctx, cfg)
	if err != nil {
		t.Errorf("error creating new mux: %s", err)
		return
	}
	_, ok := mfs.handlers["ipfs"]
	if !ok {
		t.Errorf("expected 'ipfs' filesystem, got none")
	}
	_, ok = mfs.handlers["httpfs"]
	if !ok {
		t.Errorf("expected 'ipfs' filesystem, got none")
	}
	_, ok = mfs.handlers["localfs"]
	if !ok {
		t.Errorf("expected 'ipfs' filesystem, got none")
	}
	_, ok = mfs.handlers["mem"]
	if !ok {
		t.Errorf("expected 'ipfs' filesystem, got none")
	}
}

func TestOptSetIPFSPathWithConfig(t *testing.T) {
	// test empty muxConfig
	o := &[]MuxConfig{
		{
			Type:   "ipfs",
			Config: map[string]interface{}{"fsRepoPath": "bad/path"},
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
	gotPath, ok := ipfscfg.Config["fsRepoPath"]
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
	gotPath, ok := ipfscfg.Config["fsRepoPath"]
	if !ok {
		t.Errorf("expected ipfs map[string]interface config to have field 'fsRepoPath', but it does not")
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
		{Type: "ipfs", Config: map[string]interface{}{"fsRepoPath": path}},
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
