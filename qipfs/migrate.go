package qipfs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	migrate "github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
	"github.com/otiai10/copy"
)

const configFilename = "config"

// ErrNeedMigration indicates a migration must be run before qipfs can be used
var ErrNeedMigration = fmt.Errorf(`ipfs: need datastore migration`)

// InternalizeIPFSRepo takes an ipfsRepoPath and newRepoPath
// it creates a copy of the ipfs repo, moves it to the
// new repo path and migrates that repo
// it cleans up any tmp directories made, and removes
// the new repo if any errors occur
// IT DOES NOT REMOVE THE ORIGINAL REPO
func InternalizeIPFSRepo(ipfsRepoPath, newRepoPath string) error {
	// bail if a config file already exists at new repo path
	if _, err := os.Stat(filepath.Join(newRepoPath, configFilename)); err == nil {
		return fmt.Errorf("repo already exists at new location")
	}

	// create temp directory into which we will copy the
	// ipfs directory
	tmpDir, err := ioutil.TempDir(os.TempDir(), "ipfs_temp")
	if err != nil {
		return fmt.Errorf("error creating temp directory: %w", err)
	}

	rollback := func() {
		if ipfsRepoPath != newRepoPath {
			os.RemoveAll(newRepoPath)
		}
	}

	defer func() {
		os.RemoveAll(tmpDir)
		if rollback != nil {
			rollback()
		}
	}()

	// make a back up of the ipfs repo
	err = copy.Copy(ipfsRepoPath, tmpDir)
	if err != nil {
		return fmt.Errorf("error backing up ipfs repo: %w", err)
	}

	// migrate the copied ipfs repo
	os.Setenv("IPFS_PATH", tmpDir)
	if err := Migrate(); err != nil {
		return fmt.Errorf("error migrating ipfs repo: %w", err)
	}

	// move migrated repo to new location
	if err := os.Rename(tmpDir, newRepoPath); err != nil {
		return fmt.Errorf("error moving repo onto new path: %w", err)
	}

	if err := migrateToInternalIPFSConfig(ipfsRepoPath, newRepoPath); err != nil {
		return fmt.Errorf("internalizing repo configuration: %w", err)
	}

	rollback = nil
	return nil
}

// Migrate runs an IPFS fsrepo migration
func Migrate() error {
	err := migrate.RunMigration(fsrepo.RepoVersion)
	if err != nil {
		fmt.Println("The migrations of fs-repo failed:")
		fmt.Printf("  %s\n", err)
		fmt.Println("If you think this is a bug, please file an issue and include this whole log output.")
		fmt.Println("  https://github.com/ipfs/fs-repo-migrations")
		return err
	}
	return nil
}

func migrateToInternalIPFSConfig(repoReadPath, repoWritePath string) error {
	cfg := map[string]interface{}{}
	data, err := ioutil.ReadFile(filepath.Join(repoReadPath, configFilename))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	cfg["Addresses"] = map[string]interface{}{
		"Swarm": []interface{}{
			"/ip4/0.0.0.0/tcp/0",
			"/ip6/::/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic",
			"/ip6/::/udp/0/quic",
		},
		"Announce":   []interface{}{},
		"NoAnnounce": []interface{}{},
		"API":        "/ip4/127.0.0.1/tcp/0",
		"Gateway":    "/ip4/127.0.0.1/tcp/0",
	}

	data, err = json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(repoWritePath, configFilename), data, 0666)
}
