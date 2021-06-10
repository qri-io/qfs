package localfs

import (
	"context"
	"testing"

	"github.com/qri-io/qfs"
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

func TestSizeFile(t *testing.T) {
	ctx := context.Background()
	fs, err := NewFS(map[string]interface{}{
		"PWD": ".",
	})

	if err != nil {
		t.Fatal(err)
	}

	f, err := fs.Get(ctx, "testdata/text.txt")
	if err != nil {
		t.Fatal(err)
	}

	expect := int64(12)
	got := f.(qfs.SizeFile).Size()
	if expect != got {
		t.Errorf("size mismatch. want: %d got: %d", expect, got)
	}
}
