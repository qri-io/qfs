package ipfs_filestore

import (
	"fmt"

	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	migrate "github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
)

// ErrNeedMigration is an
var ErrNeedMigration = fmt.Errorf(`ipfs: need datastore migration`)

// Migrate runs an IPFS fsrepo migration
func Migrate() error {
	err := migrate.RunMigration(fsrepo.RepoVersion)
	if err != nil {
		fmt.Println("The migrations of fs-repo failed:")
		fmt.Printf("  %s\n", err)
		fmt.Println("If you think this is a bug, please file an issue and include this whole log output.")
		fmt.Println("  https://github.com/ipfs/fs-repo-migrations")
	}
	return err
}
