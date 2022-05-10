package distro

type Distro interface {
	GetName() string
	GetReleaseName() string
	GetModeState(mode string) (string, error)
	GetCachePath() string
	GetCacheSaltStackSourcePath() string
	GetSaltstackPillars() (pillars map[string]string)
	Download(dir string) error
}
