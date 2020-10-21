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

// MerkelizeCallback is a function that's called when a given path has been
// written to the content addressed filesystem
type MerkelizeCallback func(ctx context.Context, f qfs.File, merkelizedPaths map[string]string) (io.Reader, error)

// MerkelizeHook configures a callback function to be executed on a saved
// file, at a specific point in the merkelization process
type MerkelizeHook struct {
	// path of file to fire on
	inputFilename string
	// file for delayed hook calls
	file qfs.File
	// once mutex for callback execution
	once sync.Once
	// slice of pre-merkelized paths that need to be saved before the hook
	// can be called
	requiredPaths []string
	// function to call
	callback MerkelizeCallback
}

// NewMerkelizeHook creates
func NewMerkelizeHook(inputFilename string, cb MerkelizeCallback, requiredPaths ...string) *MerkelizeHook {
	return &MerkelizeHook{
		inputFilename: inputFilename,
		requiredPaths: requiredPaths,
		callback:      cb,
	}
}

func (h *MerkelizeHook) hasRequiredPaths(merkelizedPaths map[string]string) bool {
	for _, p := range h.requiredPaths {
		if _, ok := merkelizedPaths[p]; !ok {
			return false
		}
	}
	return true
}

func (h *MerkelizeHook) callAndAdd(ctx context.Context, adder Adder, f qfs.File, merkelizedPaths map[string]string) (err error) {
	h.once.Do(func() {
		log.Debugf("calling merkelizeHook path=%s merkelized=%#v", h.inputFilename, merkelizedPaths)
		var r io.Reader
		r, err = h.callback(ctx, f, merkelizedPaths)
		if err != nil {
			return
		}
		if err = adder.AddFile(ctx, qfs.NewMemfileReader(h.inputFilename, r)); err != nil {
			return
		}
	})

	return err
}

// WriteWithHooks writes a file or directory to a given filestore using
// merkelization hooks
// failed writes are rolled back with delete requests for all added files
func WriteWithHooks(ctx context.Context, fs Filestore, root qfs.File, hooks ...*MerkelizeHook) (string, error) {
	var (
		finalPath       string
		waitingHooks    []*MerkelizeHook
		doneCh          = make(chan error, 0)
		addedCh         = make(chan AddedFile, 1)
		merkelizedPaths = map[string]string{}
		tasks           = 0
	)

	hookMap, err := processHookPaths(hooks)
	if err != nil {
		return "", err
	}

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
			log.Debugf("visiting %s waitingHooks=%d", file.FullPath(), len(waitingHooks))

			for i, hook := range waitingHooks {
				if hook.hasRequiredPaths(merkelizedPaths) {
					log.Debugf("calling delayed hook: %s", hook.inputFilename)
					if err := hook.callAndAdd(ctx, adder, hook.file, merkelizedPaths); err != nil {
						return err
					}
					waitingHooks = append(waitingHooks[i:], waitingHooks[:i+1]...)
					// wait for one path to be added
					<-addedCh
				}
			}

			if hook, hookExists := hookMap[file.FullPath()]; !hookExists {
				if err := adder.AddFile(ctx, file); err != nil {
					return err
				}
				// wait for one path to be added
				<-addedCh

			} else if hook.hasRequiredPaths(merkelizedPaths) {
				log.Debugf("calling hook for path %s", file.FullPath())
				if err := hook.callAndAdd(ctx, adder, file, merkelizedPaths); err != nil {
					return err
				}
				// wait for one path to be added
				<-addedCh
			} else {
				hook.file = file
				log.Debugf("adding hook to waitlist for path %s", file.FullPath())
				waitingHooks = append(waitingHooks, hook)
			}

			return nil
		})

		for i, hook := range waitingHooks {
			if !hook.hasRequiredPaths(merkelizedPaths) {
				doneCh <- fmt.Errorf("reequirements for hook %q were never met", hook.inputFilename)
				return
			}

			log.Debugf("calling delayed hook: %s", hook.inputFilename)
			if err := hook.callAndAdd(ctx, adder, hook.file, merkelizedPaths); err != nil {
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

func processHookPaths(hooks []*MerkelizeHook) (hookMap map[string]*MerkelizeHook, err error) {
	requiredPaths := map[string]struct{}{}
	hookMap = map[string]*MerkelizeHook{}

	for _, hook := range hooks {
		if _, exists := hookMap[hook.inputFilename]; exists {
			return nil, fmt.Errorf("multiple hooks provided for path %q", hook.inputFilename)
		}
		hookMap[hook.inputFilename] = hook
		for _, p := range hook.requiredPaths {
			requiredPaths[p] = struct{}{}
		}
	}

	return hookMap, nil
}
