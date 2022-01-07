package config

import (
	"io/ioutil"

	"github.com/ekristen/cast/pkg/distro"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Release  Release         `yaml:"release,omitempty"`
	Manifest distro.Manifest `yaml:"manifest"`
}

type Release struct {
	GitHub     GitHub   `yaml:"github"`
	Header     string   `yaml:"header,omitempty"`
	Footer     string   `yaml:"footer,omitempty"`
	ExtraFiles []string `yaml:"extra_files"`
}

type GitHub struct {
	Owner string `yaml:"owner"`
	Repo  string `yaml:"repo"`
}

func New() *Config {
	return &Config{}
}

func Load(configFile string) (cfg *Config, err error) {
	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}

	return cfg, err
}
