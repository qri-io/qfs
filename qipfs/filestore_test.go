package qipfs

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/qri-io/qfs"
	"github.com/qri-io/qfs/cafs"
	"github.com/qri-io/qfs/cafs/test"
	"github.com/qri-io/qfs/qipfs/qipfs_http"
)

func TestFS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	path := InitTestRepo(t)
	defer os.RemoveAll(path)

	f, err := NewFilesystem(ctx, map[string]interface{}{
		"online": false,
		"path":   path,
	})
	if err != nil {
		t.Errorf("error creating filestore: %s", err.Error())
		return
	}

	cafs, ok := f.(cafs.Filestore)
	if !ok {
		t.Errorf("error, filesystem should be of type cafs.Filestore")
	}
	err = test.EnsureFilestoreBehavior(cafs)
	if err != nil {
		t.Errorf(err.Error())
	}

	releasingFS, ok := f.(qfs.ReleasingFilesystem)
	if !ok {
		t.Fatal("ipfs filesystem is not a qfs.ReleasingFilesystem")
	}

	cancel()
	select {
	case <-time.NewTimer(time.Millisecond * 100).C:
		t.Errorf("done didn't fire within 100ms of context cancellation")
	case <-releasingFS.Done():
	}

	if path, err := f.Put(context.Background(), qfs.NewMemfileBytes("/ipfs/foo.json", []byte(`oh hello`))); err == nil {
		t.Errorf("expected putting file into closed store to error. got none. path: %q", path)
	}
}

func TestCreatedWithAPIAddrFS(t *testing.T) {
	ctx, done := context.WithCancel(context.Background())
	defer done()

	path := InitTestRepo(t)
	defer os.RemoveAll(path)

	// create an ipfs fs with that repo
	_, err := NewFilesystem(ctx, map[string]interface{}{
		"online":    false,
		"path":      path,
		"enableAPI": true,
	})
	if err != nil {
		t.Errorf("error creating filestore: %s", err.Error())
		return
	}

	// attempt to create another filestore using the same repo
	if _, err := NewFilesystem(ctx, map[string]interface{}{
		"online": false,
		"path":   path,
	}); err == nil {
		t.Errorf("There should be a repo lock error when attempting to create another filesystem using the same repo path, however no error occured")
	}

	// create another filestore, but with a fallback api address
	cafs, err := NewFilesystem(ctx, map[string]interface{}{
		"online": false,
		"path":   path,
		"url":    "http://127.0.0.1:5001/api/v0",
	})
	if err != nil {
		t.Errorf("error creating ipfs_http filesystem: %s", err)
	}
	if _, ok := cafs.(*qipfs_http.Filestore); !ok {
		t.Errorf("returned filesystem is not of expected type `ipfs_http`")
	}
}

func BenchmarkRead(b *testing.B) {
	ctx, done := context.WithCancel(context.Background())
	defer done()

	path := filepath.Join(os.TempDir(), "ipfs_cafs_benchmark_read")

	if _, err := os.Open(filepath.Join(path, "config")); os.IsNotExist(err) {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			b.Errorf("error creating temp dir: %s", err.Error())
			return
		}

		if err := InitRepo(path, ""); err != nil {
			b.Errorf("error intializing repo: %s", err.Error())
			return
		}

		defer os.RemoveAll(path)
	}

	f, err := NewFilesystem(ctx, map[string]interface{}{
		"online": false,
		"path":   path,
	})
	if err != nil {
		b.Errorf("error creating filestore: %s", err.Error())
		return
	}

	egFilePath := "testdata/complete.json"
	data, err := ioutil.ReadFile(egFilePath)
	if err != nil {
		b.Errorf("error reading temp file data: %s", err.Error())
		return
	}

	key, err := f.Put(ctx, qfs.NewMemfileBytes(filepath.Base(egFilePath), data))
	if err != nil {
		b.Errorf("error putting example file in store: %s", err.Error())
		return
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gotf, err := f.Get(ctx, key)
		if err != nil {
			b.Errorf("iteration %d error getting key: %s", i, err.Error())
			break
		}

		gotData, err := ioutil.ReadAll(gotf)
		if err != nil {
			b.Errorf("iteration %d error reading data bytes: %s", i, err.Error())
			break
		}

		if len(data) != len(gotData) {
			b.Errorf("iteration %d byte length mistmatch. expected: %d, got: %d", i, len(data), len(gotData))
			break
		}

		un := map[string]interface{}{}
		if err := json.Unmarshal(gotData, &un); err != nil {
			b.Errorf("iteration %d error unmarshaling data: %s", i, err.Error())
			break
		}
	}
}

func TestPinsetDifference(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	path := InitTestRepo(t)
	defer os.RemoveAll(path)

	f, err := NewFilesystem(ctx, map[string]interface{}{"path": path})
	if err != nil {
		t.Fatalf("creating filestore: %s", err.Error())
		return
	}

	fs := f.(*Filestore)
	filter := map[string]struct{}{}
	pinsch, err := fs.PinsetDifference(ctx, filter)
	if err != nil {
		t.Error(err)
	}

	got := map[string]struct{}{}
	for path := range pinsch {
		got[path] = struct{}{}
	}

	expect := map[string]struct{}{
		"/ipld/QmQPeNsJPyVWPFDVHb77w8G42Fvo15z4bG2X8D2GhfbSXc": {},
		"/ipld/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn": {},
	}
	if diff := cmp.Diff(expect, got); diff != "" {
		t.Errorf("result mismatch (-want +got):\n%s", diff)
	}

	filter = map[string]struct{}{
		"/ipld/QmQPeNsJPyVWPFDVHb77w8G42Fvo15z4bG2X8D2GhfbSXc": {},
	}
	pinsch, err = fs.PinsetDifference(ctx, filter)
	if err != nil {
		t.Error(err)
	}

	got = map[string]struct{}{}
	for path := range pinsch {
		got[path] = struct{}{}
	}

	expect = map[string]struct{}{
		"/ipld/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn": {},
	}
	if diff := cmp.Diff(expect, got); diff != "" {
		t.Errorf("result mismatch (-want +got):\n%s", diff)
	}
}

// TestDisableBootstrap should test that the DisableBootstrap option
// does not permanently remove the bootstrap addrs from the ipfs config
func TestDisableBootstrap(t *testing.T) {
	path := InitTestRepo(t)
	// inspect initial configuration, ensure there are bootstrap addrs
	data, err := ioutil.ReadFile(filepath.Join(path, "config"))
	if err != nil {
		t.Fatalf("error opening config file: %s", err)
	}

	cfg := map[string]interface{}{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("error unmarshaling config: %s", err)
	}
	bootstrapAddrs := cfg["Bootstrap"].([]interface{})
	if len(bootstrapAddrs) == 0 {
		t.Fatalf("error: config starts with no Bootstrap addrs")
	}

	// create a new fs with the disableBootstrap flag
	ctx, cancel := context.WithCancel(context.Background())
	_, err = NewFilesystem(ctx, map[string]interface{}{
		"path":             path,
		"disableBootstrap": true,
	})
	if err != nil {
		t.Fatalf("error creating new filesystem: %s", err)
	}
	cancel()

	// inspect the configuration again, make sure the Bootstrap addrs have not been removed
	data, err = ioutil.ReadFile(filepath.Join(path, "config"))
	if err != nil {
		t.Fatalf("error opening config file after creating new filesystem: %s", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("error unmarshaling config after creating new filesystem: %s", err)
	}
	bootstrapAddrs = cfg["Bootstrap"].([]interface{})
	if len(bootstrapAddrs) == 0 {
		t.Fatalf("error: creating new filesystem with the 'disableBootstrap' flag has removed the underlying Bootstrap addresses")
	}
}

// InitTestRepo creates a repo at the given path
func InitTestRepo(t *testing.T) string {
	path, err := ioutil.TempDir("", t.Name())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		t.Fatalf("error creating temp dir: %s", err.Error())
	}
	if err := InitRepo(path, ""); err != nil {
		t.Fatalf("error intializing repo: %s", err.Error())
	}

	return path
}
