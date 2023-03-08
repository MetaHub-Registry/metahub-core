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

type Rule struct {
	Name  string `yaml:"name"`
	Tag   string `yaml:"tag"`
	Type  string `yaml:"type"`
	Op    string `yaml:"op"`
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

func (r Rule) ToTypesRule() types.Rule {
	return types.Rule{
		Name:  r.Name,
		Tag:   r.Tag,
		Type:  r.Type,
		Op:    r.Op,
		Key:   r.Key,
		Value: r.Value,
	}
}

func FindRules(cacheDir string) (rules []Rule, err error) {
	ctx := context.Background()
	rules = []Rule{}
	files, err := FindYamlFiles(cacheDir)
	if err != nil {
		err = errors.Wrapf(err, "Error finding yaml files in %s", cacheDir)
		return
	}
	// Parse each filter file
	for _, ffile := range files {
		r, e := LoadRule(ctx, ffile)
		if err != nil {
			err = errors.Wrapf(e, "error parsing rule file %s", ffile)
			return
		}
		rules = append(rules, r)
	}
	return
}

func LoadRule(ctx context.Context, yamlfile string) (r Rule, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "config",
		"func":    "LoadRule",
		"file":    yamlfile,
	})
	tr := otel.Tracer("config")
	ctx, span := tr.Start(ctx, "LoadProfile")
	_ = ctx
	span.SetAttributes(attribute.Key("path").String(yamlfile))
	defer span.End()
	re := regexp.MustCompile(profileRegex)
	m := utils.ReSubMatchMap(re, yamlfile)
	name := m["dir"]
	tag := m["file"]
	yamlContent, err := ioutil.ReadFile(yamlfile)
	if err != nil {
		l.Errorf("Cannot load manifest config file #%v ", err)
		return Rule{}, err
	}
	l.Debug("attempting readManifestConfig()")
	r, err = readRuleConfig(yamlContent)
	if err != nil {
		l.Errorf("Cannot read manifest config file #%v ", err)
		return Rule{}, err
	}
	if r.Name != "" && r.Name != name {
		err = errors.Errorf("rule name '%s' does not match directory '%s'; please consider removing name/tag, will be derived by name", r.Name, name)
		return
	}
	if r.Tag != "" && r.Tag != tag {
		err = errors.Errorf("rule tag '%s' does not match filename '%s'; please consider removing name/tag, will be derived by name", r.Tag, tag)
		return
	}
	r.Name = name
	r.Tag = tag
	return r, nil
}

// readRuleConfig reads (and returns) a profile
func readRuleConfig(yamlStr []byte) (r Rule, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "config",
		"func":    "readRuleConfig",
	})
	// First unmarshall into generic structure
	r = Rule{}
	err = yaml.Unmarshal(yamlStr, &r)
	if err != nil {
		l.Fatalf("Unmarshal: %v\n", err)
		return
	}
	return
}
