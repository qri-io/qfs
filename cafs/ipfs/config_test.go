package ipfs_filestore

import (
	"os"
	"testing"
	"path/filepath"
)

func TestMoveIPFSRepoOnToQriPath(t *testing.T) {
	path := filepath.Join(os.TempDir(), "ipfs_repo_move_test")
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		t.Errorf("error creating temp dir: %s", err.Error())
		return
	}
	defer os.RemoveAll(path)

	qriPath := filepath.Join(path, ".qri")
	ipfsPath := filepath.Join(path, ".ipfs")
	
	if err := os.MkdirAll(qriPath, os.ModePerm); err != nil {
		t.Errorf("error creating temp qri dir: %s", err.Error())
		return	
	}
	if err := os.MkdirAll(ipfsPath, os.ModePerm); err != nil {
		t.Errorf("error creating temp ipfs dir: %s", err.Error())
		return	
	}

	err := os.Setenv("QRI_PATH", qriPath)
	if err != nil {
		t.Errorf("failed to set up QRI_PATH: %s", err.Error())
		return
	}

	cfg := &StoreCfg{
		FsRepoPath: ipfsPath,
	}

	if _, err := os.Stat(qriPath); os.IsNotExist(err) {
		t.Errorf("error: qriPath directory was not created")
		return
	}
	if _, err := os.Stat(ipfsPath); os.IsNotExist(err) {
		t.Errorf("error: ipfsPath directory was not created")
		return
	}

	if err := MoveIPFSRepoOnToQriPath(cfg); err != nil {
		t.Errorf("MoveIPFSRepoOnToQriPath error: %s", err)
		return
	}

	newIPFSPath := filepath.Join(qriPath, ".ipfs")
	if _, err := os.Stat(ipfsPath); !os.IsNotExist(err) {
		t.Errorf("old ipfs dir should not exist")
		return
	}

	if _, err := os.Stat(newIPFSPath); os.IsNotExist(err) {
		t.Errorf("IPFS repo was not moved onto the new IPFS path: %s", newIPFSPath)
		return
	}
	if cfg.FsRepoPath != newIPFSPath {
		t.Errorf("FsRepoPath in Store Config object was not changed")
		return
	}
}