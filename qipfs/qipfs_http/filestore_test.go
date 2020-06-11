package qipfs_http

import (
	"testing"
)

func TestMapToConfig(t *testing.T) {
	m := map[string]interface{}{
		"url": "/path/to/api/url",
	}
	cfg, err := mapToConfig(m)
	if err != nil {
		t.Errorf("error converting map string interface to config struct: %s", err)
	}
	if cfg.URL != m["url"] {
		t.Errorf("expected cfg.url to be %s, got %s", m["url"], cfg.URL)
	}
}
