package config

import (
	"context"
	"fmt"
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

type Manipulation struct {
	Label        string        `yaml:"label"`
	ManipuLayers []ManipuLayer `yaml:"layers"`
}

func FindManipulations(cacheDir string) (mps []Manipulation, err error) {
	ctx := context.Background()
	mps = []Manipulation{}
	files, err := FindYamlFiles(cacheDir)
	if err != nil {
		err = errors.Wrapf(err, "Error finding yaml files in %s", cacheDir)
		return
	}
	// Parse each filter file
	for _, ffile := range files {
		mp, e := LoadManipulation(ctx, ffile)
		if err != nil {
			err = errors.Wrapf(e, "error parsing rule file %s", ffile)
			return
		}
		mps = append(mps, mp)
	}
	return
}

func LoadManipulation(ctx context.Context, yamlfile string) (mp Manipulation, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "config",
		"func":    "LoadManipulation",
		"file":    yamlfile,
	})
	tr := otel.Tracer("config")
	ctx, span := tr.Start(ctx, "LoadManipulation")
	_ = ctx
	span.SetAttributes(attribute.Key("path").String(yamlfile))
	defer span.End()
	re := regexp.MustCompile(`.*/manipulations` + fileRegex)
	var label string
	if m, e := utils.ReSubMatchMap(re, yamlfile); e != nil {
		err = errors.Wrapf(e, "Error parsing file %s", yamlfile)
		return
	} else {
		name := m["dir"]
		fname := m["file"]
		label = fmt.Sprintf("%s#%s", name, fname)
	}
	yamlContent, err := ioutil.ReadFile(yamlfile)
	if err != nil {
		l.Errorf("Cannot load manifest config file #%v ", err)
		return Manipulation{}, err
	}
	l.Debug("attempting readManifestConfig()")
	mp, err = readManipulationConfig(yamlContent)
	if err != nil {
		l.Errorf("Cannot read manifest config file #%v ", err)
		return Manipulation{}, err
	}

	if mp.Label != "" && mp.Label != label {
		err = errors.Errorf("Manipulation label '%s' does not match directory and file name '%s'; please consider removing it, so ot will be derived by path/filename", mp.Label, label)
		return
	}
	mp.Label = label
	return mp, nil
}

// readManipulationConfig reads (and returns) a manipulation
func readManipulationConfig(yamlStr []byte) (mp Manipulation, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "config",
		"func":    "readManipulationConfig",
	})
	// First unmarshall into generic structure
	mp = Manipulation{}
	err = yaml.Unmarshal(yamlStr, &mp)
	if err != nil {
		err = errors.Wrapf(err, "error unmarshalling yaml")
		l.Error(err)
		return mp, err
	}
	return
}

func (cfgMp Manipulation) ToTypesManipulation() (tmp types.Manipulation) {
	tmp = types.Manipulation{
		Label: cfgMp.Label,
	}
	for _, layer := range cfgMp.ManipuLayers {
		tl := layer.ToTypesManipuLayer()
		tmp.ManipuLayers = append(tmp.ManipuLayers, tl)
	}
	return
}
