package ipfs_filestore

import (
	"fmt"
	"io/ioutil"
	"os"

	migrate "github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
	"github.com/otiai10/copy"
)

// ErrNeedMigration is an
var ErrNeedMigration = fmt.Errorf(`ipfs: need datastore migration`)

// RunMigrations takes an ipfsRepoPath and newRepoPath
// it creates a copy of the ipfs repo, moves it to the
// new repo path and migrates that repo
// it cleans up any tmp directories made, and removes
// the new repo if any errors occur
// IT DOES NOT REMOVE THE ORIGINAL REPO
func RunMigrations(ipfsRepoPath, newRepoPath string) error {
	// create temp directory into which we will copy the
	// ipfs directory
	tmpDir, err := ioutil.TempDir(os.TempDir(), "ipfs_temp")
	if err != nil {
		return err
	}
	rollback := func() {
		os.RemoveAll(newRepoPath)
	}
	defer func() {
		os.RemoveAll(tmpDir)
		if rollback != nil {
			rollback()
		}
	}()

	err = copy.Copy(ipfsRepoPath, tmpDir)
	if err != nil {
		return err
	}

	err = MoveIPFSRepoOntoPath(tmpDir, newRepoPath)
	if err != nil {
		return err
	}
	rollback = nil
	return nil
}

// Migrate runs an IPFS fsrepo migration
func Migrate(prevVersion, newVersion int) error {
	err := migrate.RunMigration(newVersion)
	if err != nil {
		fmt.Println("The migrations of fs-repo failed:")
		fmt.Printf("  %s\n", err)
		fmt.Println("If you think this is a bug, please file an issue and include this whole log output.")
		fmt.Println("  https://github.com/ipfs/fs-repo-migrations")
		return err
	}
	return nil
}
