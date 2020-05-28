package ipfs_filestore

import (
	"testing"
)

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
