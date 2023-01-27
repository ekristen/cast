package distro

import (
	"bytes"
	"github.com/Masterminds/sprig/v3"
	"html/template"
	"strings"

	"gopkg.in/yaml.v3"
)

func ParseManifest(contents []byte) (m *Manifest, err error) {
	if err := yaml.Unmarshal(contents, &m); err != nil {
		return nil, err
	}

	return m, nil
}

type Manifest struct {
	Version     int           `json:"version" yaml:"version" default:"2"`
	Name        string        `json:"name,omitempty" yaml:"name,omitempty"`
	Base        string        `json:"base_dir" yaml:"base_dir" default:"."`
	Modes       []Mode        `json:"modes" yaml:"modes"`
	Saltstack   Saltstack     `json:"saltstack,omitempty" yaml:"saltstack,omitempty"`
	SupportedOS []SupportedOS `json:"supported_os,omitempty" yaml:"supported_os,omitempty"`
}

type Mode struct {
	Name        string `json:"name" yaml:"name"`
	State       string `json:"state" yaml:"state"`
	Deprecated  bool   `json:"deprecated" yaml:"deprecated,omitempty"`
	Replacement string `json:"replacement" yaml:"replacement,omitempty"`
	Default     bool   `json:"default" yaml:"default,omitempty"`
}

type SupportedOS struct {
	ID       string `yaml:"id"`
	Release  string `yaml:"release,omitempty"`
	Codename string `yaml:"codename,omitempty"`
}

type Saltstack struct {
	Pillars map[string]string `json:"pillars,omitempty" yaml:"pillars,omitempty"`
}

func (m *Manifest) Render(data interface{}) error {
	for name, val := range m.Saltstack.Pillars {
		if strings.HasSuffix(name, "_template") {
			tmpl, err := template.New("pillar_template").Funcs(sprig.FuncMap()).Parse(val)
			if err != nil {
				return err
			}

			var content bytes.Buffer
			if err := tmpl.Execute(&content, data); err != nil {
				return err
			}

			m.Saltstack.Pillars[strings.TrimSuffix(name, "_template")] = content.String()
			delete(m.Saltstack.Pillars, name)
		}
	}

	return nil
}
