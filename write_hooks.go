package qfs

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
)

// WriteHook is a function that's called when a given path has been
// written to the content addressed filesystem
type WriteHook func(ctx context.Context, f File, added map[string]string) (io.Reader, error)

// hookFile configures a callback function to be executed on a saved
// file, at a specific point in the merkelization process
type hookFile struct {
	// file for delayed hook calls
	File
	// once mutex for callback execution
	once sync.Once
	// slice of pre-merkelized paths that need to be saved before the hook
	// can be called
	requiredPaths []string
	// function to call
	callback WriteHook
}

// Assert hookFile implements HookFile at compile time
var _ WriteHookFile = (*hookFile)(nil)

// WriteHookFile is a file that can hook into the merkelization process, affecting
// contents as contents are being rendered immutable
type WriteHookFile interface {
	File
	RequiredPaths() []string
	HasRequiredPaths(paths map[string]string) bool
	CallAndAdd(ctx context.Context, adder Adder, added map[string]string) error
}

// NewWriteHookFile wraps a File with a hook & set of sibling / child dependencies
func NewWriteHookFile(file File, cb WriteHook, requiredPaths ...string) File {
	if file.IsDirectory() {
		panic("cannot create a WriteHookFile with a Directory")
	}

	return &hookFile{
		File:          file,
		requiredPaths: requiredPaths,
		callback:      cb,
	}
}

func (h *hookFile) RequiredPaths() []string {
	return h.requiredPaths
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
		log.Debugf("calling hook path=%s merkelized=%#v", h.FullPath(), merkelizedPaths)
		var r io.Reader
		r, err = h.callback(ctx, h.File, merkelizedPaths)
		if err != nil {
			return
		}
		if err = adder.AddFile(ctx, NewMemfileReader(h.FullPath(), r)); err != nil {
			return
		}
	})

	return err
}

// WriteWithHooks writes a file or directory to a given filestore using
// merkelization hooks
// failed writes are rolled back with delete requests for all added files
func WriteWithHooks(ctx context.Context, fs Filesystem, root File) (string, error) {
	var (
		finalPath       string
		waitingHooks    []WriteHookFile
		doneCh          = make(chan error, 0)
		addedCh         = make(chan AddedFile, 1)
		merkelizedPaths = map[string]string{}
	)

	addFS, ok := fs.(AddingFS)
	if !ok {
		return "", ErrNotAddingFS
	}

	adder, err := addFS.NewAdder(ctx, true, true)
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
			// finalPath = ao.Path
			addedCh <- ao
		}
	}()

	go func() {
		err := Walk(root, func(file File) error {
			if file.IsDirectory() {
				return nil
			}

			log.Debugf("visiting %s waitingHooks=%d added=%v", file.FullPath(), len(waitingHooks), merkelizedPaths)

			for i, whf := range waitingHooks {
				if whf.HasRequiredPaths(merkelizedPaths) {
					log.Debugf("calling delayed hook: %s", whf.FileName())
					if err := whf.CallAndAdd(ctx, adder, merkelizedPaths); err != nil {
						log.Debugf("delayed WriteHookFile error=%s", err)
						return err
					}
					waitingHooks = append(waitingHooks[i:], waitingHooks[:i+1]...)
					// wait for one path to be added
					<-addedCh
				}
			}

			if whf, isAHook := file.(WriteHookFile); isAHook {
				if whf.HasRequiredPaths(merkelizedPaths) {
					if err := whf.CallAndAdd(ctx, adder, merkelizedPaths); err != nil {
						log.Debugf("WriteHookFile error=%s", err)
						return err
					}
					// wait for one path to be added
					<-addedCh
				} else {
					log.Debugf("adding hook to waitlist for path %s", file.FullPath())
					waitingHooks = append(waitingHooks, whf)
				}
				return nil
			}

			if err := adder.AddFile(ctx, file); err != nil {
				log.Debugf("adder.AddFile error=%s", err)
				return err
			}
			// wait for one path to be added
			<-addedCh

			return nil
		})

		if err != nil {
			log.Debugf("walk error=%s", err)
			doneCh <- err
		}

		for i, hook := range waitingHooks {
			if !hook.HasRequiredPaths(merkelizedPaths) {
				missed := make([]string, 0, len(hook.RequiredPaths()))
				for _, path := range hook.RequiredPaths() {
					if _, ok := merkelizedPaths[path]; !ok {
						missed = append(missed, path)
					}
				}

				doneCh <- fmt.Errorf("requirements for hook %q were never met. missing required paths: %s", hook.FullPath(), strings.Join(missed, ", "))
				return
			}

			log.Debugf("calling delayed hook: %s", hook.FullPath())
			if err := hook.CallAndAdd(ctx, adder, merkelizedPaths); err != nil {
				doneCh <- err
			}
			waitingHooks = append(waitingHooks[i:], waitingHooks[:i+1]...)
		}

		finalPath, err = adder.Finalize()
		if err != nil {
			doneCh <- err
		}

		doneCh <- nil
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
