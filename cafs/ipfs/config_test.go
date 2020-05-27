package ipfs_filestore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMoveIPFSRepoOnToQriPath(t *testing.T) {
	path := filepath.Join(os.TempDir(), "ipfs_repo_move_test")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Errorf("error creating temp dir: %s", err.Error())
		return
	}
	defer os.RemoveAll(path)

	qriPath := filepath.Join(path, ".qri")
	ipfsPath := filepath.Join(path, ".ipfs")

	if err := os.MkdirAll(qriPath, 0755); err != nil {
		t.Errorf("error creating temp qri dir: %s", err.Error())
		return
	}
	if err := os.MkdirAll(ipfsPath, 0755); err != nil {
		t.Errorf("error creating temp ipfs dir: %s", err.Error())
		return
	}

	if _, err := os.Stat(qriPath); os.IsNotExist(err) {
		t.Errorf("error: qriPath directory was not created")
		return
	}
	if _, err := os.Stat(ipfsPath); os.IsNotExist(err) {
		t.Errorf("error: ipfsPath directory was not created")
		return
	}

	newIpfsPath := filepath.Join(qriPath, ".ipfs")

	if err := MoveIPFSRepoOntoPath(ipfsPath, newIpfsPath); err != nil {
		t.Errorf("MoveIPFSRepoOntoPath error: %s", err)
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
}

func TestMapToConfig(t *testing.T) {
	m := map[string]interface{}{
		"fsRepoPath": "/path/to/repo",
		"enableAPI":  true,
		"apiAddr":    "/api/addr",
	}
	cfg, err := mapToConfig(m)
	if err != nil {
		t.Errorf("error converting map string interface to config struct: %s", err)
	}
	if cfg.FsRepoPath != m["fsRepoPath"] {
		t.Errorf("expected cfg.FsRepoPath to be %s, got %s", m["fsRepoPath"], cfg.FsRepoPath)
	}
	if cfg.EnableAPI != m["enableAPI"] {
		t.Errorf("expected cfg.EnableAPI to be %t, got %t", m["enableAPI"], cfg.EnableAPI)
	}
	if cfg.APIAddr != m["apiAddr"] {
		t.Errorf("expected cfg.APIAddr to be %s, got %s", m["apiAddr"], cfg.APIAddr)
	}
}
