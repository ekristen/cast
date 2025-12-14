package state

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// InstallState represents the saved state for a single distro installation
type InstallState struct {
	DistroName string `yaml:"distro_name"`
	Version    string `yaml:"version"`
	Mode       string `yaml:"mode"`
}

// State represents the persisted state file
type State struct {
	Installations map[string]InstallState `yaml:"installations"`
}

var (
	stateMu sync.Mutex
)

// GetStatePath returns the path to the state file
func GetStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "cast", "state.yaml"), nil
}

// Load reads the state from disk
func Load() (*State, error) {
	stateMu.Lock()
	defer stateMu.Unlock()

	s := &State{
		Installations: make(map[string]InstallState),
	}

	statePath, err := GetStatePath()
	if err != nil {
		return s, err
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, s); err != nil {
		return nil, err
	}

	if s.Installations == nil {
		s.Installations = make(map[string]InstallState)
	}

	return s, nil
}

// Save writes the state to disk
func (s *State) Save() error {
	stateMu.Lock()
	defer stateMu.Unlock()

	statePath, err := GetStatePath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}

	return os.WriteFile(statePath, data, 0644)
}

// GetInstallState retrieves the saved state for a distro
func (s *State) GetInstallState(distroKey string) (InstallState, bool) {
	state, ok := s.Installations[distroKey]
	return state, ok
}

// SetInstallState saves the state for a distro
func (s *State) SetInstallState(distroKey string, installState InstallState) {
	s.Installations[distroKey] = installState
}
