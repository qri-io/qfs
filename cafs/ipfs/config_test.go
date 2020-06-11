package ipfs_filestore

import (
	"testing"
)

func TestMapToConfig(t *testing.T) {
	m := map[string]interface{}{
		"path": "/path/to/repo",
		"url":  "http://localhost:5001",

		"enableAPI":    true,
		"enablePubSub": true,
	}
	cfg, err := mapToConfig(m)
	if err != nil {
		t.Errorf("error converting map string interface to config struct: %s", err)
	}
	if cfg.Path != m["path"] {
		t.Errorf("expected cfg.path to be %s, got %s", m["path"], cfg.Path)
	}
	if cfg.EnableAPI != m["enableAPI"] {
		t.Errorf("expected cfg.EnableAPI to be %t, got %t", m["enableAPI"], cfg.EnableAPI)
	}
	if cfg.EnablePubSub != m["enablePubSub"] {
		t.Errorf("expected cfg.EnableAPI to be %t, got %t", m["enablePubSub"], cfg.EnablePubSub)
	}
	if cfg.URL != m["url"] {
		t.Errorf("expected cfg.URL to be %s, got %s", m["apiAddr"], cfg.URL)
	}
}
