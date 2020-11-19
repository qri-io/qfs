package qfs

import (
	"context"
	"errors"
)

// ErrNotAddingFS is the canonical error to return when the AddingFS extension
// interface is required but not available
var ErrNotAddingFS = errors.New("this filesystem doesn't support batched adding")

// AddedFile reports on the results of adding a file to the store
type AddedFile struct {
	Path  string
	Name  string
	Bytes int64
	Hash  string
	Size  string
}

// AddingFS is an interface for filesystems that support batched adding
type AddingFS interface {
	// NewAdder allocates an Adder instance for adding files to the filestore
	// Adder gives a higher degree of control over the file adding process at the
	// cost of being harder to work with.
	// "pin" is a flag for recursively pinning this object
	// "wrap" sets weather the top level should be wrapped in a directory
	NewAdder(ctx context.Context, pin, wrap bool) (Adder, error)
}

// Adder is the interface for adding files to a Filestore. The addition process
// is parallelized. Implementers must make all required AddFile calls, then call
// Close to finalize the addition process. Progress can be monitored through the
// Added() channel, which emits once per written file
type Adder interface {
	// AddFile adds a file or directory of files to the store
	// this function will return immediately, consumers should read
	// from the Added() channel to see the results of file addition.
	AddFile(context.Context, File) error
	// Added gives a channel to read added files from.
	Added() chan AddedFile
	// In IPFS land close calls adder.Finalize() and adder.PinRoot()
	// (files will only be pinned if the pin flag was set on NewAdder)
	Finalize() (string, error)
}
