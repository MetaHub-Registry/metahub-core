package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"
)

// modelRegex extracts <repo>/<tag> from a CE models/ path, mirroring the
// collections loader: models/qnib/minicpm5/latest.yaml → repo=qnib/minicpm5, tag=latest.
var modelRegex = `.*models\/(?P<repo>.*)\/(?P<tag>[0-9a-z\-\_\.]+)\.(yaml|yml)`

// Model is the declarative CE authoring shape for a servable model (the `models/`
// type). It compiles to the internal composition (weights + runtime collection +
// config), resolved per requesting profile.
type Model struct {
	Name    string       `yaml:"name"`
	Tag     string       `yaml:"tag"`
	Runtime string       `yaml:"runtime"` // a collections/ ref, resolved per profile
	Quants  []ModelQuant `yaml:"quants"`
	Latest  ModelLatest  `yaml:"latest"`
	Config  ModelConfig  `yaml:"config"`
}

// ModelQuant is one servable quantization: tag is the pull tag, source the ORAS
// image metahub-admin repackages into a model layer.
type ModelQuant struct {
	Tag    string `yaml:"tag"`
	Source string `yaml:"source"`
}

// ModelLatest is the `:latest` policy: a default (e.g. "best-fit") with optional
// per-profile / per-rule quant overrides (resolved by the shared override grammar).
type ModelLatest struct {
	Default   string               `yaml:"default"`
	Overrides []ModelQuantOverride `yaml:"overrides,omitempty"`
}

// ModelQuantOverride pins a quant for a profile alias or a hardware rule. Exactly
// one of Profile / Rule is set.
type ModelQuantOverride struct {
	Profile string `yaml:"profile,omitempty"`
	Rule    string `yaml:"rule,omitempty"`
	Quant   string `yaml:"quant"`
}

// ModelConfig is the inline config: a default (a map of abstract keys, or the
// string "auto" to derive) plus per-profile / per-rule overrides.
type ModelConfig struct {
	Default   any                   `yaml:"default"`
	Overrides []ModelConfigOverride `yaml:"overrides,omitempty"`
}

// ModelConfigOverride patches the config for a profile alias or a hardware rule.
type ModelConfigOverride struct {
	Profile string         `yaml:"profile,omitempty"`
	Rule    string         `yaml:"rule,omitempty"`
	Config  map[string]any `yaml:"config"`
}

// Validate checks the required shape: a runtime, at least one quant (each with a
// tag + source), and exactly one selector (profile XOR rule) per override.
func (m Model) Validate() error {
	if m.Runtime == "" {
		return fmt.Errorf("model %q: runtime is required", m.Name)
	}
	if len(m.Quants) == 0 {
		return fmt.Errorf("model %q: at least one quant is required", m.Name)
	}
	for _, q := range m.Quants {
		if q.Tag == "" || q.Source == "" {
			return fmt.Errorf("model %q: each quant needs a tag and a source (got %+v)", m.Name, q)
		}
	}
	for _, o := range m.Latest.Overrides {
		if (o.Profile == "") == (o.Rule == "") {
			return fmt.Errorf("model %q: latest override needs exactly one of profile/rule (got %+v)", m.Name, o)
		}
		if o.Quant == "" {
			return fmt.Errorf("model %q: latest override needs a quant (got %+v)", m.Name, o)
		}
	}
	for _, o := range m.Config.Overrides {
		if (o.Profile == "") == (o.Rule == "") {
			return fmt.Errorf("model %q: config override needs exactly one of profile/rule (got %+v)", m.Name, o)
		}
	}
	return nil
}

func normalizeModelFname(m *Model, mfile string) (repo, tag string, err error) {
	re := regexp.MustCompile(modelRegex)
	sub, e := utils.ReSubMatchMap(re, mfile)
	if e != nil {
		return "", "", e
	}
	repo, tag = sub["repo"], sub["tag"]
	if m.Name == "" {
		m.Name = repo
	}
	if m.Tag == "" {
		m.Tag = tag
	}
	if m.Name != repo || m.Tag != tag {
		return "", "", fmt.Errorf("model file states name:%s tag:%s which mismatches path repo:%s tag:%s (you can omit name/tag)", m.Name, m.Tag, repo, tag)
	}
	return repo, tag, nil
}

// FindModels walks a CE tree for models/ files and returns them keyed by
// `<repo>:<tag>` (library/ prefixed when the repo has no org segment), mirroring
// FindManifestCollections.
func FindModels(ctx context.Context, cacheDir string) (models map[string]Model, err error) {
	l := logrus.WithFields(logrus.Fields{"package": "config", "func": "FindModels", "cacheDir": cacheDir})
	tr := otel.Tracer("config")
	ctx, span := tr.Start(ctx, "FindModels")
	span.SetAttributes(attribute.Key("path").String(cacheDir))
	defer span.End()
	models = make(map[string]Model)
	files, err := FindYamlFiles(cacheDir)
	if err != nil {
		return models, err
	}
	for _, mfile := range files {
		m, e := LoadModel(ctx, mfile)
		if e != nil {
			l.Errorf("failed to load model file %s: %s", mfile, e)
			continue
		}
		// A single unusable file must not discard the whole scan: callers
		// (metahub-admin parse models) treat a returned error as "skip this
		// directory", which silently dropped every model when an example
		// template lived under models/.
		repo, tag, e := normalizeModelFname(&m, mfile)
		if e != nil {
			l.Errorf("skipping model file %s: %s", mfile, e)
			continue
		}
		if e := m.Validate(); e != nil {
			l.Errorf("skipping invalid model file %s: %s", mfile, e)
			continue
		}
		uri := repo + ":" + tag
		if strings.Count(repo, "/") == 0 {
			uri = "library/" + uri
		}
		if _, ok := models[uri]; ok {
			l.Warnf("duplicate model uri %s, not added again", uri)
			continue
		}
		models[uri] = m
	}
	return models, nil
}

// LoadModel reads + parses a single models/ yaml file.
func LoadModel(ctx context.Context, yamlfile string) (Model, error) {
	_ = ctx
	content, err := ioutil.ReadFile(yamlfile)
	if err != nil {
		return Model{}, err
	}
	m := Model{}
	if err := yaml.Unmarshal(content, &m); err != nil {
		return Model{}, err
	}
	return m, nil
}
