package config

import "github.com/ekristen/cast/pkg/distro"

type Config struct {
	Manifest distro.Manifest `yaml:"manifest"`
	Release  Release         `yaml:"release,omitempty"`
}

type Release struct {
	ExtraFiles []string `yaml:"extra_files"`
}
