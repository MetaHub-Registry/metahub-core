package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"strings"

	"gopkg.in/yaml.v3"

	"github.com/metahub-registry/metahub-core/pkg/utils"
)

var (
	defaultArch   = "amd64"
	defaultOS     = "linux"
	manifestRegex = `.*collections\/(?P<repo>.*)\/(?P<tag>[0-9a-z\-\_\.]+)\.(yaml|yml)`
)

type ManifestCollection struct {
	Name string `yaml:"name"`
	Tag  string `yaml:"tag"`
	//Annotations map[string]string `yaml:"annotations"`
	Manifests []Manifest `yaml:"manifests"`
}

func NewManifestCollection(name, tag string) (mc ManifestCollection) {
	mc.Name = name
	mc.Tag = tag
	return
}

type Platform struct {
	Os       string   `yaml:"os"`
	Arch     string   `yaml:"arch"`
	Variant  string   `yaml:"variant"`
	Features []string `yaml:"features"`
}

// FindManifestConfigs looks for yml/yaml manifest config files in some root
func FindManifestCollections(ctx context.Context, cacheDir string) (cfgs map[string]ManifestCollection, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":  "config",
		"func":     "FindManifestCollections",
		"cacheDir": cacheDir,
	})
	tr := otel.Tracer("config")
	ctx, span := tr.Start(ctx, "FindManifestCollections")
	span.SetAttributes(attribute.Key("path").String(cacheDir))
	defer span.End()
	cfgs = make(map[string]ManifestCollection)
	files, err := FindYamlFiles(cacheDir)

	// Read in each manifest config
	for _, mfile := range files {
		l.Debugf("Reading manifest config file %s\n", mfile)
		ctx, fileSpan := tr.Start(ctx, mfile)
		defer fileSpan.End()
		r := manifestRegex
		re := regexp.MustCompile(r)
		m := utils.ReSubMatchMap(re, mfile)
		tag := m["tag"]
		repo := m["repo"]
		manifest, err := LoadManifestConfig(ctx, mfile)
		if err != nil {
			l.Warnf("Issue reading %s: %x, skipping.\n", mfile, err)
			continue
		}
		if manifest.Name == "" {
			manifest.Name = repo
		}
		if manifest.Tag == "" {
			manifest.Tag = tag
		}
		// Required to have name and tag
		if manifest.Name != repo || manifest.Tag != tag {
			l.Warnf("Manifest %s defines the repo (%s) and tag (%s)via path", mfile, repo, tag)
			err = fmt.Errorf("the collection file states name:%s and tag:%s which is a missmatch. You can skip the name and tag definition btw", manifest.Name, manifest.Tag)
			return cfgs, err

		}
		uri := repo + ":" + tag
		_, manifestSpan := tr.Start(ctx, uri)
		defer manifestSpan.End()
		// If we don't have a registry, must be library
		// TODO we don't read the registry currently (assumed all docker hub)
		if strings.Count(uri, "/") == 0 {
			uri = "library/" + uri
		}

		// We aren't allowed to add the same one twice, issue a warning
		_, ok := cfgs[uri]
		if ok {
			l.Warnf("Duplicate uri %s found, will not be added again.", uri)
			continue
		}
		l.Debugf("Added: %s", uri)
		cfgs[uri] = manifest
		manifestSpan.End()
		fileSpan.End()
	}
	return
}

// readManifestConfig reads (and returns) a manifest config
func readManifestConfig(yamlStr []byte) (col ManifestCollection, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "config",
		"func":    "readManifestConfig",
	})
	// First unmarshall into generic structure
	col = ManifestCollection{}
	err = yaml.Unmarshal(yamlStr, &col)
	if err != nil {
		l.Fatalf("Unmarshal: %v\n", err)
		return
	}
	for i, m := range col.Manifests {
		if m.Platform.OS == "" {
			m.Platform.OS = defaultOS
		}
		if m.Platform.Architecture == "" {
			m.Platform.Architecture = defaultArch
		}
		col.Manifests[i] = m

	}
	return
}

// LoadManifestConfig loads a yaml manifest
func LoadManifestConfig(ctx context.Context, yamlfile string) (ManifestCollection, error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "config",
		"func":    "LoadManifestConfig",
		"file":    yamlfile,
	})
	tr := otel.Tracer("config")
	ctx, span := tr.Start(ctx, "LoadManifestConfig")
	_ = ctx
	span.SetAttributes(attribute.Key("path").String(yamlfile))
	defer span.End()
	yamlContent, err := ioutil.ReadFile(yamlfile)
	if err != nil {
		l.Errorf("Cannot load manifest config file #%v ", err)
		return ManifestCollection{}, err
	}
	l.Debug("attempting readManifestConfig()")
	return readManifestConfig(yamlContent)
}
