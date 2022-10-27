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
	imgNameRegex        = regexp.MustCompile(`^(?P<repo>[\w\/\-\.\_]+)(:|@)(?P<ref>.*)$`)
	singleImg           = regexp.MustCompile(`^(?P<repo>[\w\-\.\_]+)(:|@)(?P<ref>.*)$`)
	singleImgNoTagRegex = regexp.MustCompile(`^(?P<repo>[\w\.]+)$`)
	regs                = []string{"docker.io", "quay.io", "registry.gitlab.com", "public.ecr.aws"}
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
	if name == "" {
		err = fmt.Errorf("%s is not a valid image name", name)
		return
	}
	foundReg := false
	for _, r := range regs {
		if strings.HasPrefix(name, r) {
			reg = r
			name = strings.TrimPrefix(name, r+"/")
			foundReg = true
			break
		}
	}
	if !foundReg {
		l.Warnf("No registry found in '%s' - fallback to docker.io", name)
		reg = "docker.io"
	}
	_ = l
	if singleImgNoTagRegex.MatchString(name) {
		l.Warnf("singleImgNoTagRegex -> name: '%s'", name)
		m := ReSubMatchMap(singleImgNoTagRegex, name)
		repo = fmt.Sprintf("library/%s", m["repo"])
		ref = "latest"
		return
	}
	if singleImg.MatchString(name) {
		m := ReSubMatchMap(imgNameRegex, name)
		repo = fmt.Sprintf("library/%s", m["repo"])
		ref = m["ref"]
		return
	}
	if imgNameRegex.MatchString(name) {
		m := ReSubMatchMap(imgNameRegex, name)
		repo = m["repo"]
		ref = m["ref"]
		return
	}
	err = fmt.Errorf("%s is not a valid image name", name)
	return
}
