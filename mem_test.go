package qfs_test

import (
	"context"
	"testing"

	"github.com/qri-io/qfs"
	qfsspec "github.com/qri-io/qfs/spec"
)

func TestMemFS(t *testing.T) {
	qfsspec.AssertFilesystemSpec(t, func(ctx context.Context) qfs.Filesystem {
		return qfs.NewMemFS()
	})
}
