package localfs

import (
	"testing"
)

func TestMapToConfig(t *testing.T) {
	m := map[string]interface{}{
		"PWD": "/path/to/working/directory",
	}
	cfg, err := mapToConfig(m)
	if err != nil {
		t.Errorf("error converting map string interface to config struct: %s", err)
	}
	if cfg.PWD != m["PWD"] {
		t.Errorf("expected cfg.PWD to be %s, got %s", m["PWD"], cfg.PWD)
	}
}
