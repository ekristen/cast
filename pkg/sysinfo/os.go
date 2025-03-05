package sysinfo

// Original Code from: https://github.com/zcalusic/sysinfo/blob/master/os.go
// Modified for use here

import (
	"bufio"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

// OS information.
type OS struct {
	Name         string `json:"name,omitempty"`
	Vendor       string `json:"vendor,omitempty"`
	Version      string `json:"version,omitempty"`
	Release      string `json:"release,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	Codename     string `json:"codename,omitempty"`
}

var (
	rePrettyName = regexp.MustCompile(`^PRETTY_NAME=(.*)$`)
	reID         = regexp.MustCompile(`^ID=(.*)$`)
	reVersionID  = regexp.MustCompile(`^VERSION_ID=(.*)$`)
	reCodename   = regexp.MustCompile(`^VERSION_CODENAME=(.*)$`)
	reUbuntu     = regexp.MustCompile(`[\( ]([\d\.]+)`)
	reCentOS     = regexp.MustCompile(`^CentOS( Linux)? release ([\d\.]+) `)
	reRedHat     = regexp.MustCompile(`[\( ]([\d\.]+)`)
)

func GetOSInfo() (osinfo *OS) {
	osinfo = &OS{}
	// This seems to be the best and most portable way to detect OS architecture (NOT kernel!)
	if _, err := os.Stat("/lib64/ld-linux-x86-64.so.2"); err == nil {
		osinfo.Architecture = "x86_64"
	} else if _, err := os.Stat("/lib/ld-linux.so.2"); err == nil {
		osinfo.Architecture = "i386"
	} else if _, err := os.Stat("/lib/ld-linux-aarch64.so.1"); err == nil {
		osinfo.Architecture = "arm64"
	}

	f, err := os.Open("/etc/os-release")
	if err != nil {
		return
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if m := rePrettyName.FindStringSubmatch(s.Text()); m != nil {
			osinfo.Name = strings.Trim(m[1], `"`)
		} else if m := reID.FindStringSubmatch(s.Text()); m != nil {
			osinfo.Vendor = strings.Trim(m[1], `"`)
		} else if m := reVersionID.FindStringSubmatch(s.Text()); m != nil {
			osinfo.Release = strings.Trim(m[1], `"`)
		} else if m := reCodename.FindStringSubmatch(s.Text()); m != nil {
			osinfo.Codename = strings.Trim(m[1], `"`)
		} else if m := reUbuntu.FindStringSubmatch(s.Text()); m != nil {
			osinfo.Version = strings.Trim(m[1], `"`)
		}
	}

	switch osinfo.Vendor {
	case "debian":
		osinfo.Release = readFile("/etc/debian_version")
	case "centos":
		if release := readFile("/etc/centos-release"); release != "" {
			if m := reCentOS.FindStringSubmatch(release); m != nil {
				osinfo.Release = m[2]
			}
		}
	case "rhel":
		if release := readFile("/etc/redhat-release"); release != "" {
			if m := reRedHat.FindStringSubmatch(release); m != nil {
				osinfo.Release = m[1]
			}
		}
		if osinfo.Release == "" {
			if m := reRedHat.FindStringSubmatch(osinfo.Name); m != nil {
				osinfo.Release = m[1]
			}
		}
	}

	return osinfo
}

func readFile(path string) string {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}
