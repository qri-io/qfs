package ipfs_filestore

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/qri-io/qfs"
	"github.com/qri-io/qfs/cafs"
	"github.com/qri-io/qfs/cafs/ipfs_http"
	"github.com/qri-io/qfs/cafs/test"
)

var _ cafs.Fetcher = (*Filestore)(nil)

func TestFS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	path := filepath.Join(os.TempDir(), "ipfs_cafs_test")
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		t.Errorf("error creating temp dir: %s", err.Error())
		return
	}
	defer os.RemoveAll(path)

	if err := InitRepo(path, ""); err != nil {
		t.Errorf("error intializing repo: %s", err.Error())
		return
	}

	f, err := NewFS(ctx, nil, func(c *StoreCfg) {
		c.Online = false
		c.FsRepoPath = path
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

	path := filepath.Join(os.TempDir(), "ipfs_cafs_test_api_addr")
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		t.Errorf("error creating temp dir: %s", err.Error())
		return
	}
	defer os.RemoveAll(path)

	// create an repo
	if err := InitRepo(path, ""); err != nil {
		t.Errorf("error intializing repo: %s", err.Error())
		return
	}

	// create an ipfs fs with that repo
	_, err := NewFS(ctx, nil, func(c *StoreCfg) {
		c.Online = false
		c.FsRepoPath = path
		c.EnableAPI = true
	})
	if err != nil {
		t.Errorf("error creating filestore: %s", err.Error())
		return
	}

	// attempt to create another filestore using the same repo
	if _, err := NewFS(ctx, nil, func(c *StoreCfg) {
		c.Online = false
		c.FsRepoPath = path
	}); err == nil {
		t.Errorf("There should be a repo lock error when attempting to create another filesystem using the same repo path, however no error occured")
	}

	// create another filestore, but with a fallback api address
	cafs, err := NewFS(ctx, nil, func(c *StoreCfg) {
		c.Online = false
		c.FsRepoPath = path
		c.APIAddr = "127.0.0.1:5001/api/v0/swarm/peers"
	})
	if err != nil {
		t.Errorf("error creating ipfs_http filesystem: %s", err)
	}
	if _, ok := cafs.(*ipfs_http.Filestore); !ok {
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

	f, err := NewFS(ctx, nil, func(c *StoreCfg) {
		c.Online = false
		c.FsRepoPath = path
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
