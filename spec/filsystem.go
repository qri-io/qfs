package spec

import (
	"context"
	"errors"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/qri-io/qfs"
)

// NewEmptyFSFunc is a constructor function for creating an empty filesystem
type NewEmptyFSFunc func(ctx context.Context) qfs.Filesystem

// AssertFilesystemSpec is a test suite that ensures a filesystem implementation
// conforms to expected behaviours
func AssertFilesystemSpec(t *testing.T, constructor NewEmptyFSFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fs := constructor(ctx)

	t.Run("files", func(t *testing.T) {
		if err := fs.Delete(ctx, "/example/file"); !errors.Is(err, qfs.ErrNotFound) {
			t.Errorf("trying to delete a file should return a not found error. got: %v", err)
		}

		contents := "hello world!"
		file := qfs.NewMemfileBytes("/example/file", []byte(contents))
		name, err := fs.WriteFile(ctx, file)
		if err != nil {
			t.Fatalf("writing a file shouldn't return an error. got: %q", err)
		}
		assertNamePathPrefix(t, fs, name)
		assertHas(t, fs, name)

		file, err = fs.OpenFile(ctx, name)
		if err != nil {
			t.Fatalf("opening a just added file shouldn't error. got: %q", err)
		}

		fi, err := file.Stat()
		if err != nil {
			t.Fatalf("getting stats for an open file shouldn't error. got: %q", err)
		}

		if fi.IsDir() {
			t.Error("file stats for file shouldn't return true for .IsDir()")
		}
		if file.IsLinkedData() {
			t.Error("file shouldn't return true for .IsLinkedData()")
		}

		data, err := ioutil.ReadAll(file)
		if err != nil {
			t.Errorf("reading opened file should not error. got: %q", err)
		} else if contents != string(data) {
			t.Errorf("read file contents mismatch. want %q got %q", contents, string(data))
		}

		if err := fs.Delete(ctx, name); err != nil {
			t.Errorf("deleting written file shouldn't error. got: %q", err)
		}
		assertNotHas(t, fs, name)
	})

	t.Run("linked_data", func(t *testing.T) {
		data := map[string]interface{}{
			"string": "I'm a string",
			"float":  float64(2.3),
			"bool":   true,
			"nil":    nil,
			"map": map[string]interface{}{
				"map_key": "map_value",
			},
			"array": []interface{}{
				float64(1),
				float64(2),
				float64(3),
			},
		}
		file, err := qfs.NewLinkedDataFile("/example/linkedData", data)
		if err != nil {
			t.Fatalf("unexpected error creating linkedData file: %q", err)
		}

		name, err := fs.WriteFile(ctx, file)
		if err != nil {
			t.Errorf("writing linkedData file shouldn't error. got: %q", err)
		}
		assertNamePathPrefix(t, fs, name)
		assertHas(t, fs, name)

		file, err = fs.OpenFile(ctx, name)
		if err != nil {
			t.Fatalf("opening a just added file shouldn't error. got: %q", err)
		}

		fi, err := file.Stat()
		if err != nil {
			t.Fatalf("getting stats for an open file shouldn't error. got: %q", err)
		}

		if fi.IsDir() {
			t.Error("file stats for file shouldn't return true for .IsDir()")
		}
		if !file.IsLinkedData() {
			t.Error("file must return true for .IsLinkedData()")
		}

		got, err := file.LinkedData()
		if err != nil {
			t.Errorf("getting linked data from file shouldn't error. got: %q", err)
		}

		if diff := cmp.Diff(data, got); diff != "" {
			t.Errorf("resulting data mismatch. (-want +got):\n%s", diff)
		}

		if err := fs.Delete(ctx, name); err != nil {
			t.Errorf("deleting written linked data file shouldn't error. got: %q", err)
		}
		assertNotHas(t, fs, name)
	})

	t.Run("linked_data_write_derefed_link", func(t *testing.T) {
		data := map[string]interface{}{
			"string": "I'm a string",
			"link_with_reference": &qfs.Link{
				Ref: "/foreign/file/system",
			},
			"link_with_value": &qfs.Link{
				Value: map[string]interface{}{
					"this_is": "some other data",
				},
			},
		}
		file, err := qfs.NewLinkedDataFile("/spec/linkedData", data)
		if err != nil {
			t.Fatalf("unexpected error creating linkedData file: %q", err)
		}

		name, err := fs.WriteFile(ctx, file)
		if err != nil {
			t.Errorf("writing linkedData file shouldn't error. got: %q", err)
		}
		assertNamePathPrefix(t, fs, name)
		assertHas(t, fs, name)

		file, err = fs.OpenFile(ctx, name)
		if err != nil {
			t.Fatalf("opening a just added file shouldn't error. got: %q", err)
		}

		fi, err := file.Stat()
		if err != nil {
			t.Fatalf("getting stats for an open file shouldn't error. got: %q", err)
		}

		if fi.IsDir() {
			t.Error("file stats for file shouldn't return true for .IsDir()")
		}
		if !file.IsLinkedData() {
			t.Error("file must return true for .IsLinkedData()")
		}

		gotIface, err := file.LinkedData()
		if err != nil {
			t.Errorf("getting linked data from file shouldn't error. got: %q", err)
		}

		got, ok := gotIface.(map[string]interface{})
		if !ok {
			t.Errorf("expected returned linked data type to be map[string]interface{}, got: %T", gotIface)
		}

		if link, ok := got["link_with_value"].(*qfs.Link); ok {
			if link.Value != nil {
				t.Error(`expected opened "link_with_value" to be a reference. Instead, link.Value field != nil`)
			}
			if link.Ref == "" {
				t.Error(`expected "link_with_value" Ref field to be set.`)
			}

			expectLink := data["link_with_value"].(*qfs.Link)
			expectLink.Ref = link.Ref
			expectLink.Value = nil
		} else {
			t.Errorf("expected linked data file 'link_with_data' value to be a link type. got: %T", got["link_with_value"])
		}

		if diff := cmp.Diff(data, got); diff != "" {
			t.Errorf("resulting data mismatch. (-want +got):\n%s", diff)
		}

		if err := fs.Delete(ctx, name); err != nil {
			t.Errorf("deleting written linked data file shouldn't error. got: %q", err)
		}
		assertNotHas(t, fs, name)
	})

	t.Run("directories", func(t *testing.T) {
		dir := qfs.NewMemdir("/spec/empty/dir")

		name, err := fs.WriteFile(ctx, dir)
		if err != nil {
			t.Errorf("writing an empty directory shouldn't error. got: %q", err)
		}
		assertNamePathPrefix(t, fs, name)
		assertHas(t, fs, name)

		dir, err = fs.OpenFile(ctx, name)
		if err != nil {
			t.Errorf("opening empty directoy shouldn't error. got: %q for name: %q", err, name)
		} else {
			dfi, err := dir.Stat()
			if err != nil {
				t.Errorf("empty directory stat shouldn't error. got: %q", err)
			}
			if !dfi.IsDir() {
				t.Error("opened empty directory didn't return true for .Stat().IsDir")
			}
		}

		petsData := `suzie,oliver,spot`
		dir = qfs.NewMemdir("/spec/directories",
			qfs.NewMemdir("documents",
				qfs.NewMemfileBytes("pets.txt", []byte(petsData)),
				qfs.NewMemfileBytes("friends.txt", []byte(`Lebron James,James Comey,Kylie Jenner,SOPHIE`)),
			),
			qfs.NewMemfileBytes("groceries.json", []byte(`["apples","milk","eggs"]`)),
		)

		name, err = fs.WriteFile(ctx, dir)
		if err != nil {
			t.Errorf("writing directory should not error. got: %q", err)
		}
		assertNamePathPrefix(t, fs, name)
		assertHas(t, fs, name)

		petsName := name + "/documents/pets.txt"
		f, err := fs.OpenFile(ctx, petsName)
		if err != nil {
			t.Errorf("opening nested document should not error. got: %w", err)
		} else {
			pets, err := ioutil.ReadAll(f)
			if err != nil {
				t.Errorf("unexpected error reading %q file: %q", petsName, err)
			}
			if petsData != string(pets) {
				t.Errorf("pets.txt file contents mismatch. want: %q, got: %q", petsData, string(pets))
			}
			if err := f.Close(); err != nil {
				t.Errorf("unexpected error closing file: %q", err)
			}
		}
	})
}

func assertNamePathPrefix(t *testing.T, fs qfs.Filesystem, name string) {
	t.Helper()
	pre := qfs.NamePrefix(fs)
	if !strings.HasPrefix(name, pre) {
		t.Errorf("filsystem must have %q prefix. got: %q", pre, name)
	}
}

func assertHas(t *testing.T, fs qfs.Filesystem, name string) {
	t.Helper()
	ok, err := fs.Has(context.Background(), name)
	if err != nil {
		t.Errorf("fs.Has(filename) shouldn't error. got: %q", err)
	} else if !ok {
		t.Errorf("fs.Has() for a file that was just written should return true. got false")
	}
}

func assertNotHas(t *testing.T, fs qfs.Filesystem, name string) {
	t.Helper()
	ok, err := fs.Has(context.Background(), name)
	if err != nil {
		t.Errorf("fs.Has(filename) shouldn't error. got: %q", err)
	} else if ok {
		t.Errorf("fs.Has() for a file that was just deleted should return false. got true")
	}
}
