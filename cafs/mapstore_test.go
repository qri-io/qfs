package cafs

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/qri-io/qfs"
)

func TestPutFileAtKey(t *testing.T) {
	m := NewMapstore()
	ctx := context.Background()

	input := qfs.NewMemfileBytes("my_file.txt", []byte("hello there"))
	if err := m.PutFileAtKey(ctx, "/map/AnyKey", input); err != nil {
		t.Fatal(err)
	}

	output, err := m.Get(ctx, "/map/AnyKey/my_file.txt")
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadAll(output)
	if err != nil {
		t.Fatal(err)
	}
	actual := string(content)

	expect := "hello there"
	if diff := cmp.Diff(expect, actual); diff != "" {
		t.Errorf("result mismatch (-want +got):\n%s", diff)
	}
}
