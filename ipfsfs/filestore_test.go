package ipfsfs

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ipfs/go-ipfs/core"
	"github.com/qri-io/qfs"
	"github.com/qri-io/qfs/cafs"
	"github.com/qri-io/qfs/cafs/test"
	"github.com/qri-io/value"
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

	f, err := NewFilestore(func(c *StoreCfg) {
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

func TestPutValues(t *testing.T) {
	tr, cleanup := newTestRunner(t)
	defer cleanup()

	ds := map[string]interface{}{
		"meta": value.NewResolvedLink(
			"meta.json",
			map[string]interface{}{
				"title":   "I'm a title",
				"homeUrl": value.NewLink("https://qri.io"), // note homeUrl is unresolved, should be stored as a string
			}),
		"transform": map[string]interface{}{
			"syntax": "starlark",
			"script": qfs.NewMemfileBytes("tf.star", []byte(`# starlark script`)),
		},
		"body": qfs.NewMemfileBytes("body.json", []byte(`{"some":"body data"}`)),
	}

	f := qfs.NewMemfile("dataset.json", ds)
	path, err := tr.FS.Put(tr.Ctx, f)
	if err != nil {
		t.Error(err)
	}
	t.Log(path)

	res, err := tr.FS.Get(tr.Ctx, path)
	if err != nil {
		t.Fatal(err)
	}

	expect := map[interface{}]interface{}{
		"meta": value.NewLink("bafy2bzacedfbu3whofebydm6rnilvgtvc4ercx7gxussjgtcrvpyhz5j44o6s"),
		"transform": map[interface{}]interface{}{
			"syntax": "starlark",
			"script": value.NewLink("QmQsvVMRALob2PZArwTUeBhUSJ7otAdv4cZJyb38LdYHwq"),
		},
		"body": value.NewLink("QmQRLygmdEa7kMRwKAWrWj2G1y868PZUWvy9gjtQsLuTkz"),
	}

	linkComparer := cmp.Comparer(func(a, b value.Link) bool {
		return a.Path() == b.Path()
	})

	if diff := cmp.Diff(expect, res.Value(), linkComparer); diff != "" {
		t.Errorf("result mismatch. (-want +got):\n%s", diff)
	}

	var resolver value.Resolver = tr.FS
	metaVal, err := resolver.Resolve(tr.Ctx, expect["meta"].(value.Link))
	if err != nil {
		t.Error(err)
	}

	t.Logf("%#v", metaVal)
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

	f, err := NewFilestore(func(c *StoreCfg) {
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

type testRunner struct {
	Ctx  context.Context
	Node *core.IpfsNode
	FS   *Filestore
}

func newTestRunner(t *testing.T) (tr *testRunner, cleanup func()) {
	ctx := context.Background()
	node, _, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	cleanup = func() {}

	fs, err := NewFilestore(func(cfg *StoreCfg) {
		cfg.Node = node
	})
	if err != nil {
		t.Fatal(err)
	}

	return &testRunner{
		Ctx:  ctx,
		Node: node,
		FS:   fs,
	}, cleanup
}
