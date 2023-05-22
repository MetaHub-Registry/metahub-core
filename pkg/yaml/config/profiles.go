package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"
)

var (
	profileRegex = `.*(profiles|rules)(\/(?P<dir>.*))?\/(?P<file>[0-9a-z\-\_\.]+)\.(yaml|yml)`
)

type Profile struct {
	Name          string         `yaml:"name"`
	Tag           string         `yaml:"tag"`
	Rule          string         `yaml:"rule"`
	Manipulations []Manipulation `yaml:"manipulations"`
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
	m, e := utils.ReSubMatchMap(re, yamlfile)
	if e != nil {
		err = errors.Wrapf(e, "Error parsing file %s", yamlfile)
		return
	}
	if _, ok := m["file"]; !ok {
		err = errors.Errorf("no tag found in filename %s, that we need", yamlfile)
		return
	}
	var name, tag string
	if _, ok := m["dir"]; ok {
		name = m["dir"]
		tag = m["file"]
	} else {
		name = m["file"]
	}
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
		//fmt.Printf("### Regexp: \n%s\n", re.String())
		//fmt.Printf("### yamlfile: \n%s\n", yamlfile)
		err = errors.Errorf("profile name '%s' does not match filename '%s'; please consider removing name/tag, will be derived by name", p.Name, name)
		return
	}
	if p.Tag != "" && p.Tag != tag {
		err = errors.Errorf("profile tag %s does not match filename %s; please consider removing name/tag, will be derived by name", p.Tag, tag)
		return
	}
	label := fmt.Sprintf("%s#%s", name, tag)
	if name == "" {
		label = tag
	}
	logrus.Warnf("Profile %s", label)
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
