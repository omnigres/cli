package orb

import (
	"errors"
	"fmt"
	"github.com/omnigres/cli/internal/fileutils"
	"github.com/spf13/viper"
	"path/filepath"
)

type Config struct {
	Orbs  []OrbCfg
	Image ImageConfig
	path  string
}

type OrbCfg struct {
	Name       string
	Extensions []string
}

type ImageConfig struct {
	Name   string
	Digest string `mapstructure:",omitempty"`
}

func NewConfig() *Config {
	return &Config{Image: ImageConfig{Name: "ghcr.io/omnigres/omnigres-17"}}
}

func (c *Config) Save() (err error) {
	if c.path != "" {
		c.SaveAs(c.path)
	} else {
		err = errors.New("Config has no path")
	}
	return
}
func (c *Config) SaveAs(path string) (err error) {
	v := viper.New()
	v.SetConfigName("omnigres")
	v.SetConfigType("yaml")
	v.AddConfigPath(path)

	v.Set("orbs", c.Orbs)
	v.Set("image", c.Image)

	err = fileutils.CreateIfNotExists(filepath.Join(path, "omnigres.yaml"), false)
	if err != nil {
		return
	}

	err = v.WriteConfig()
	if err != nil {
		return
	}
	return
}

func LoadConfig(path string) (cfg *Config, err error) {
	v := viper.New()
	v.SetConfigFile(filepath.Join(path, "omnigres.yaml"))
	err = v.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("omnigres.yaml not found in %s. Try setting a different workspace with -w or running omnigres init to create a new project config.", path)
	}

	cfg = NewConfig()
	cfg.path = path
	err = v.Unmarshal(cfg)
	if err != nil {
		return
	}
	return
}
