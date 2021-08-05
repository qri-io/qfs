package qipfs

import (
	"errors"

	"github.com/ipfs/go-ipfs/core"
	"github.com/mitchellh/mapstructure"
)

// ErrNoRepoPath is returned when no repo path is provided in the config
var ErrNoRepoPath = errors.New("must provide a repo path to initialize an ipfs filesystem")

// StoreCfg configures the datastore
type StoreCfg struct {
	// embed options for creating a node
	core.BuildCfg
	// optionally just supply a node. will override everything
	Node *core.IpfsNode
	// path to a local filesystem fs repo
	Path string
	// URL is an ipfs http api address, used as a fallback if we cannot
	// config an ipfs filesystem. The filesystem will instead be a `ipfs_http`
	// filesystem.
	URL string

	// weather or not to serve the local IPFS HTTP API. does not apply when
	// operating over HTTP via a URL
	EnableAPI bool
	// enable experimental IPFS pubsub service. does not apply when
	// operating over HTTP via a URL
	EnablePubSub bool
	// DisableBootstrap will remove the bootstrap addrs from the node
	DisableBootstrap bool
	// AdditionalSwarmListeningAddrs allows you to add a list of
	// addresses you want the underlying libp2p swarm to listen on
	AdditionalSwarmListeningAddrs []string
}

func mapToConfig(cfgmap map[string]interface{}) (*StoreCfg, error) {
	if cfgmap == nil {
		return DefaultConfig(""), nil
	}
	cfg := &StoreCfg{}
	if err := mapstructure.Decode(cfgmap, cfg); err != nil {
		return nil, err
	}

	if cfg.BuildCfg.ExtraOpts == nil {
		cfg.BuildCfg.ExtraOpts = map[string]bool{}
	}
	cfg.BuildCfg.ExtraOpts["pubsub"] = cfg.EnablePubSub

	return cfg, cfg.Validate()
}

// DefaultConfig results in a local node that
// attempts to draw from the default ipfs filesotre location
func DefaultConfig(path string) *StoreCfg {
	return &StoreCfg{
		BuildCfg: core.BuildCfg{
			Online: false,
		},
		Path: path,
	}
}

// Validate returns an error if the configuration fields conflict
func (cfg *StoreCfg) Validate() error {
	if cfg.Path == "" && cfg.URL == "" {
		return ErrNoRepoPath
	}
	return nil
}
