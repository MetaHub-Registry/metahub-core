package config

import (
	"context"
	"io/ioutil"
	"regexp"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"
)

var (
	profileRegex = `.*\/(?P<name>.*)\/(?P<tag>[0-9a-z\-\_\.]+)\.(yaml|yml)`
)

type Profile struct {
	Name          string   `yaml:"name"`
	Tag           string   `yaml:"tag"`
	Rule          string   `yaml:"rule"`
	Manipulations []string `yaml:"manipulations"`
}

func FindProfiles(cacheDir string) (profiles []Profile, err error) {
	ctx := context.Background()
	profiles = []Profile{}
	files, err := FindYamlFiles(cacheDir)
	if err != nil {
		err = errors.Wrapf(err, "Error finding yaml files in %s", cacheDir)
		return
	}
	// Parse each filter file
	for _, ffile := range files {
		p, e := LoadProfile(ctx, ffile)
		if err != nil {
			err = errors.Wrapf(e, "error parsing filter file %s", ffile)
			return
		}
		profiles = append(profiles, p)
	}
	return
}

func LoadProfile(ctx context.Context, yamlfile string) (p Profile, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "config",
		"func":    "LoadProfile",
		"file":    yamlfile,
	})
	tr := otel.Tracer("config")
	ctx, span := tr.Start(ctx, "LoadProfile")
	_ = ctx
	span.SetAttributes(attribute.Key("path").String(yamlfile))
	defer span.End()
	re := regexp.MustCompile(profileRegex)
	m := utils.ReSubMatchMap(re, yamlfile)
	name := m["name"]
	tag := m["tag"]
	yamlContent, err := ioutil.ReadFile(yamlfile)
	if err != nil {
		l.Errorf("Cannot load profile config file #%v ", err)
		return Profile{}, err
	}
	l.Debug("attempting readManifestConfig()")
	p, err = readProfileConfig(yamlContent)
	if err != nil {
		l.Errorf("Cannot read profile config file #%v ", err)
		return Profile{}, err
	}
	if p.Name != "" && p.Name != name {
		err = errors.Errorf("profile name %s does not match filename %s; please consider removing name/tag, will be derived by name", p.Name, name)
		return
	}
	if p.Tag != "" && p.Tag != tag {
		err = errors.Errorf("profile name %s does not match filename %s; please consider removing name/tag, will be derived by name", p.Tag, tag)
		return
	}
	logrus.Warnf("Profile %s#%s", name, tag)
	p.Name = name
	p.Tag = tag
	return p, nil
}

// readProfileConfig reads (and returns) a profile
func readProfileConfig(yamlStr []byte) (p Profile, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "config",
		"func":    "readProfileConfig",
	})
	// First unmarshall into generic structure
	p = Profile{}
	err = yaml.Unmarshal(yamlStr, &p)
	if err != nil {
		l.Fatalf("Unmarshal: %v\n", err)
		return
	}

	return
}

func (p Profile) ToProfile() (tp types.Profile, err error) {
	tp = types.Profile{
		Name:          p.Name,
		Tag:           p.Tag,
		Rule:          p.Rule,
		Manipulations: p.Manipulations,
	}
	return
}
