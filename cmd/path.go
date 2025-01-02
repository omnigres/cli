package cmd

import (
	"github.com/omnigres/cli/internal/fileutils"
	"github.com/omnigres/cli/orb"
	"os"
	"path/filepath"
)

func getOrbPath(createIfNotExists bool) (path string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	path, err = filepath.Abs(filepath.Join(cwd, workspace))
	if err != nil {
		return
	}
	if createIfNotExists {
		err = fileutils.CreateIfNotExists(path, true)
		if err != nil {
			return
		}
	}
	return
}

func getOrbCluster() (cluster orb.OrbCluster, err error) {
	var orbPath string
	orbPath, err = getOrbPath(false)
	if err != nil {
		return
	}
	var cfg *orb.Config
	cfg, err = orb.LoadConfig(orbPath)
	if err != nil {
		return
	}
	cluster, err = orb.NewDockerOrbCluster()
	if err != nil {
		return
	}
	err = cluster.Configure(orb.OrbOptions{Config: cfg, Path: orbPath})
	if err != nil {
		return
	}
	return
}
