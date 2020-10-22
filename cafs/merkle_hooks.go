package cafs

import (
	"context"
	"fmt"
	"io"
	"sync"

	logger "github.com/ipfs/go-log"
	"github.com/qri-io/qfs"
)

var log = logger.Logger("cafs")

// func init() {
// 	logger.SetLogLevel("cafs", "debug")
// }

// MerkelizeHook is a function that's called when a given path has been
// written to the content addressed filesystem
type MerkelizeHook func(ctx context.Context, f qfs.File, added map[string]string) (io.Reader, error)

// hookFile configures a callback function to be executed on a saved
// file, at a specific point in the merkelization process
type hookFile struct {
	// file for delayed hook calls
	qfs.File
	// once mutex for callback execution
	once sync.Once
	// slice of pre-merkelized paths that need to be saved before the hook
	// can be called
	requiredPaths []string
	// function to call
	callback MerkelizeHook
}

// Assert hookFile implements HookFile at compile time
var _ HookFile = (*hookFile)(nil)

// HookFile is a file that can hook into the merkelization process, affecting
// contents as contents are being rendered immutable
type HookFile interface {
	qfs.File
	HasRequiredPaths(paths map[string]string) bool
	CallAndAdd(ctx context.Context, adder Adder, added map[string]string) error
}

// NewHookFile wraps a File with a hook & set of sibling / child dependencies
func NewHookFile(file qfs.File, cb MerkelizeHook, requiredPaths ...string) qfs.File {
	return &hookFile{
		File:          file,
		requiredPaths: requiredPaths,
		callback:      cb,
	}
}

func (h *hookFile) HasRequiredPaths(merkelizedPaths map[string]string) bool {
	for _, p := range h.requiredPaths {
		if _, ok := merkelizedPaths[p]; !ok {
			log.Debugf("hook %q can't fire. waiting for %s", h.FullPath(), p)
			return false
		}
	}
	return true
}

func (h *hookFile) CallAndAdd(ctx context.Context, adder Adder, merkelizedPaths map[string]string) (err error) {
	h.once.Do(func() {
		log.Debugf("calling hookFile path=%s merkelized=%#v", h.FullPath(), merkelizedPaths)
		var r io.Reader
		r, err = h.callback(ctx, h.File, merkelizedPaths)
		if err != nil {
			return
		}
		if err = adder.AddFile(ctx, qfs.NewMemfileReader(h.FullPath(), r)); err != nil {
			return
		}
	})

	return err
}

// WriteWithHooks writes a file or directory to a given filestore using
// merkelization hooks
// failed writes are rolled back with delete requests for all added files
func WriteWithHooks(ctx context.Context, fs Filestore, root qfs.File) (string, error) {
	var (
		finalPath       string
		waitingHooks    []HookFile
		doneCh          = make(chan error, 0)
		addedCh         = make(chan AddedFile, 1)
		merkelizedPaths = map[string]string{}
		tasks           = 0
	)

	adder, err := fs.NewAdder(true, true)
	if err != nil {
		return "", err
	}

	var rollback = func() {
		log.Debug("rolling back failed write operation")
		for _, path := range merkelizedPaths {
			if err := fs.Delete(ctx, path); err != nil {
				log.Debugf("error removing path: %s: %s", path, err)
			}
		}
	}
	defer func() {
		if rollback != nil {
			log.Debug("InitDataset rolling back...")
			rollback()
		}
	}()

	go func() {
		for ao := range adder.Added() {
			log.Debugf("added name=%s hash=%s", ao.Name, ao.Path)
			merkelizedPaths[ao.Name] = ao.Path
			finalPath = ao.Path

			addedCh <- ao

			tasks--
			if tasks == 0 {
				doneCh <- nil
				return
			}
		}
	}()

	go func() {
		err := qfs.Walk(root, func(file qfs.File) error {
			tasks++
			log.Debugf("visiting %s waitingHooks=%d added=%v", file.FullPath(), len(waitingHooks), merkelizedPaths)

			for _, hf := range waitingHooks {
				if hf.HasRequiredPaths(merkelizedPaths) {
					log.Debugf("calling delayed hook: %s", hf.FileName())
					if err := hf.CallAndAdd(ctx, adder, merkelizedPaths); err != nil {
						return err
					}
					// waitingHooks = append(waitingHooks[i:], waitingHooks[:i+1]...)
					// wait for one path to be added
					<-addedCh
				}
			}

			if hf, isAHook := file.(HookFile); isAHook {
				if hf.HasRequiredPaths(merkelizedPaths) {
					log.Debugf("calling hook for path %s", file.FullPath())
					if err := hf.CallAndAdd(ctx, adder, merkelizedPaths); err != nil {
						return err
					}
					// wait for one path to be added
					<-addedCh
				} else {
					log.Debugf("adding hook to waitlist for path %s", file.FullPath())
					waitingHooks = append(waitingHooks, hf)
				}
				return nil
			}

			if err := adder.AddFile(ctx, file); err != nil {
				return err
			}
			// wait for one path to be added
			<-addedCh

			return nil
		})

		for i, hook := range waitingHooks {
			if !hook.HasRequiredPaths(merkelizedPaths) {
				doneCh <- fmt.Errorf("requirements for hook %q were never met", hook.FullPath())
				return
			}

			log.Debugf("calling delayed hook: %s", hook.FullPath())
			if err := hook.CallAndAdd(ctx, adder, merkelizedPaths); err != nil {
				doneCh <- err
			}
			waitingHooks = append(waitingHooks[i:], waitingHooks[:i+1]...)
		}

		if err != nil {
			doneCh <- err
		}
	}()

	err = <-doneCh
	if err != nil {
		log.Debugf("writing dataset: %q", err)
		return finalPath, err
	}

	log.Debugf("dataset written to filesystem. path=%q", finalPath)
	// successful execution. remove rollback func
	rollback = nil
	return finalPath, nil
}
