package config

import (
	qipfs "github.com/qri-io/qfs/cafs/ipfs"
	"github.com/qri-io/qfs/localfs"
	"github.com/qri-io/qfs/httpfs"
)

type FileSystemsConfig []FileSystemConfig

type FileSystemConfig struct {
	Type string `json:"type"`
	Options interface{} `json:"options,omitempty"`
	Source string `json:"source,omitempty"`
}

func DefaultFileSystemsConfig() *FileSystemsConfig {
	return &FileSystemsConfig{
		FileSystemConfig{
			Type: "ipfs",
			Options: qipfs.DefaultConfig(),
		},
		FileSystemConfig{
			Type: "local",
			Options: localfs.DefaultFSConfig(),
		},
		FileSystemConfig{
			Type: "http",
			Options: httpfs.DefaultFSConfig(),
		},
	}
}