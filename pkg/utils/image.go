package utils

import (
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	DefaultContainerRegistryConf = "/etc/containers/registries.conf"
)

var (
	//regs    = []string{"docker.io", "quay.io", "registry.gitlab.com", "public.ecr.aws"}
	// The registry host may contain hyphens (e.g. metahub.dev.example.com). \w
	// excludes '-', so the class must list it explicitly in both the host body and
	// the trailing label — otherwise the host is mis-split mid-name (the ".hvk"
	// label would steal characters, yielding reg="metahub.metahu").
	upRegex = `^(?P<reg>[\w\.\-]+(\:\d+|\.[\w\-]+))?\/?(?P<repo>[a-z]{1}[\w\/\.\_\-]+)(@|:)?(?P<tag>.*)?`
	defReg  = "docker.io"
)

func NormalizeRepoName(name string) (new string, err error) {
	l := log.WithFields(log.Fields{
		"package": "utils/image.go",
		"func":    "NormalizeRepoName",
		"name":    name,
	})
	_ = l
	reg, repo, ref, err := SplitImageName(name)
	if err != nil {
		return
	}
	frmt := "%s/%s:%s"
	if strings.HasPrefix(repo, "sha256") {
		frmt = "%s/%s@%s"
	}
	new = fmt.Sprintf(frmt, reg, repo, ref)
	return
}

func SplitImageName(name string) (reg, repo, ref string, err error) {
	l := log.WithFields(log.Fields{
		"package":  "registry",
		"func":     "SplitImageName",
		"org_name": name,
	})
	_ = l
	r := regexp.MustCompile(upRegex)
	switch {
	case name == "":
		err = fmt.Errorf("name is empty")
		return
	case !r.Match([]byte(name)):
		err = fmt.Errorf("name does not comply with regex '%s'", upRegex)
		return
	}

	var ok bool

	if strings.Count(name, "/") == 0 {
		if strings.Count(name, ":") == 0 {
			return defReg, "library/" + name, "latest", err

		}
		repo, ref, ok := strings.Cut(name, ":")
		if !ok {
			err = fmt.Errorf("failed to extract repo from '%s'", name)
		}
		if ref == "" {
			ref = "latest"
		}
		return defReg, "library/" + repo, ref, err

	}

	m, err := ReSubMatchMap(r, name)
	if err != nil {
		err = fmt.Errorf("failed to extract upstream from '%s': %s", name, err)
		return
	}
	reg, ok = m["reg"]
	if !ok || reg == "" {
		reg = defReg
	}
	if repo, ok = m["repo"]; !ok {
		err = fmt.Errorf("failed to extract repo from '%s'", name)
		return
	}

	if ref, ok = m["tag"]; !ok {
		err = fmt.Errorf("failed to extract tag from '%s'", name)
		return
	}
	l.Warnf("reg:'%s' / repo:'%s' / ref:'%s'", reg, repo, ref)
	return
}
