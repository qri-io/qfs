package ipfs_filestore

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/qri-io/qfs"
	"github.com/qri-io/qfs/cafs"
	"github.com/qri-io/qfs/cafs/ipfs_http"
	"github.com/qri-io/qfs/cafs/test"
)

var _ cafs.Fetcher = (*Filestore)(nil)

func init() {
	// call LoadPlugins once with the empty string b/c we only rely on standard
	// plugins
	if err := LoadPlugins(""); err != nil {
		panic(err)
	}
}

func TestFilestore(t *testing.T) {
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

	f, err := NewFilestore(nil, func(c *StoreCfg) {
		c.Online = false
		c.FsRepoPath = path
	})
	if err != nil {
		t.Errorf("error creating filestore: %s", err.Error())
		return
	}

	err = test.EnsureFilestoreBehavior(f)
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestCreatedWithAPIAddrFilestore(t *testing.T) {
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
	_, err := NewFilestore(nil, func(c *StoreCfg) {
		c.Online = false
		c.FsRepoPath = path
		c.EnableAPI = true
	})
	if err != nil {
		t.Errorf("error creating filestore: %s", err.Error())
		return
	}

	// attempt to create another filestore using the same repo
	if _, err := NewFilestore(nil, func(c *StoreCfg) {
		c.Online = false
		c.FsRepoPath = path
	}); err == nil {
		t.Errorf("There should be a repo lock error when attempting to create another filesystem using the same repo path, however no error occured")
	}

	// create another filestore, but with a fallback api address
	cafs, err := NewFilestore(nil, func(c *StoreCfg) {
		c.Online = false
		c.FsRepoPath = path
		c.APIAddr = "127.0.0.1:5001/api/v0/swarm/peers"
	})
	if err != nil {
		t.Errorf("error creating ipfs_http filesystem: %s", err)
	}
	if _, ok := cafs.(*ipfs_http.Filestore); !ok {
		t.Errorf("returned filesystem is not of expected type `ipft_http`")
	}
}

func BenchmarkRead(b *testing.B) {
	ctx := context.Background()
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

	f, err := NewFilestore(nil, func(c *StoreCfg) {
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
