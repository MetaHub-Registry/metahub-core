package config

import (
	"context"
	"io/ioutil"
	"log"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"
)

type Profile struct {
	Name    string   `yaml:"name"`
	Actions []Action `yaml:"actions"`
}

type Action struct {
	Name  string `yaml:"name"`
	Event string `yaml:"event"`

	// Args could be an interface, defined as map of strings for simplicity
	Args map[string]interface{} `yaml:"args"`
}

// readProfile reads a profile config
func readProfile(yamlStr []byte) ProfileConfig {

	// First unmarshall into generic structure
	conf := ProfileConfig{}

	err := yaml.Unmarshal(yamlStr, &conf)
	if err != nil {
		log.Fatalf("Unmarshal: %v\n", err)
	}
	if err != nil {
		log.Fatalf("Parsing Config: %v\n", err)
	}
	return conf
}

type ProfileConfig struct {
	Profile Profile `yaml:"profile"`
}

// LoadProfile loads a yaml profile
func LoadProfile(yamlfile string) ProfileConfig {
	yamlContent, err := ioutil.ReadFile(yamlfile)
	if err != nil {
		logrus.Errorf("Cannot load profile #%v ", err)
	}
	return readProfile(yamlContent)
}

func FindProfiles(ctx context.Context, cacheDir string) (prfs map[string]Profile, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":  "config",
		"func":     "FindManifestCollections",
		"cacheDir": cacheDir,
	})
	tr := otel.Tracer("config")
	ctx, span := tr.Start(ctx, "FindManifestCollections")
	span.SetAttributes(attribute.Key("path").String(cacheDir))
	defer span.End()
	prfs = make(map[string]Profile)
	files := []string{}

	for _, ext := range []string{"*.yml", "*.yaml"} {
		newFiles, err := utils.RecursiveFind(cacheDir, ext, false)
		if err != nil {
			l.Errorf("Warning, issue looking for profiles %s: %x\n", ext, err)
			continue
		}
		//l.Debugf("Add %d files with extension %s\n", len(files), newFiles)
		files = append(files, newFiles...)
	}
	// Read in each manifest config
	for _, pfile := range files {
		prf := LoadProfile(pfile)
		prfs[prf.Profile.Name] = prf.Profile
	}
	return
}
