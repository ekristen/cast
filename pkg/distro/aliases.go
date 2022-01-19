package distro

var aliases map[string]*Distro = map[string]*Distro{
	"sift": {
		Owner:   "teamdfir",
		Repo:    "sift-saltstack",
		Alias:   "sift",
		IsAlias: true,
	},
	"teamdfir/sift-saltstack": {
		Owner:   "teamdfir",
		Repo:    "sift-saltstack",
		Alias:   "sift",
		IsAlias: true,
	},
	"remnux": {
		Owner:   "remnux",
		Repo:    "salt-states",
		Alias:   "remnux",
		IsAlias: true,
	},
	"remnux/salt-states": {
		Owner:   "remnux",
		Repo:    "salt-states",
		Alias:   "remnux",
		IsAlias: true,
	},
}

var manifests map[string]*Manifest = map[string]*Manifest{
	"sift": {
		Version: 1,
		Base:    "",
		Modes: []Mode{
			{
				Name:    "desktop",
				State:   "sift.desktop",
				Default: false,
			},
			{
				Name:    "server",
				State:   "sift.server",
				Default: true,
			},
			{
				Name:        "complete",
				State:       "sift.desktop",
				Deprecated:  true,
				Replacement: "desktop",
				Default:     false,
			},
			{
				Name:        "packages-only",
				State:       "sift.server",
				Deprecated:  true,
				Replacement: "server",
				Default:     false,
			},
		},
		SupportedOS: []SupportedOS{
			{
				ID:       "ubuntu",
				Release:  "20.04",
				Codename: "focal",
			},
		},
	},
	"remnux": {
		Version: 1,
		Base:    "",
		Modes: []Mode{
			{
				Name:    "dedicated",
				State:   "remnux.dedicated",
				Default: true,
			},
			{
				Name:    "addon",
				State:   "remnux.addon",
				Default: false,
			},
			{
				Name:    "cloud",
				State:   "remnux.cloud",
				Default: false,
			},
		},
	},
}
