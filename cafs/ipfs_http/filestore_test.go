package ipfs_http

import (
	"testing"
)


func TestMapToConfig(t *testing.T) {
	m := map[string]interface{}{
		"ipfsApiUrl": "/path/to/api/url",
	}
	cfg, err := mapToConfig(m)
	if err != nil {
		t.Errorf("error converting map string interface to config struct: %s", err)
	}
	if cfg.IpfsApiUrl != m["ipfsApiUrl"] {
		t.Errorf("expected cfg.ipfsApiUrl to be %s, got %s", m["ipfsApiUrl"], cfg.IpfsApiUrl)
	}
}