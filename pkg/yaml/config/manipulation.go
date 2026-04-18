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
	"gopkg.in/yaml.v3"
)

var (
	tracer = otel.Tracer("config")
)

// Manipulation is the YAML schema for a manipulation file.
//
// Two forms are accepted:
//
// 1. Legacy flat form — each layer carries its own `rule:` and no
//    top-level defaults or variants are set. The parser converts this
//    shape into a single-layer `Variants` entry per layer and emits a
//    deprecation warning.
//
// 2. New form — top-level defaults (`Env`, `Workdir`, `User`, `Volumes`,
//    `Entrypoint`, `Cmd`, `Layers`) optionally combined with a
//    `Variants` map keyed by an expr rule string.
//
// Mixing the two (per-layer `rule:` + new top-level fields) is rejected.
type Manipulation struct {
	Label string `yaml:"label"`

	// Legacy flat layers form. The `rule` field on each layer triggers
	// the legacy code path. When any of the new top-level fields below
	// is set, this slice MUST contain only layers with an empty `rule:`.
	ManipuLayers []ManipuLayer `yaml:"layers"`

	// New extended schema.
	Env        map[string]string       `yaml:"env,omitempty"`
	Workdir    string                  `yaml:"workdir,omitempty"`
	User       string                  `yaml:"user,omitempty"`
	Volumes    []string                `yaml:"volumes,omitempty"`
	Entrypoint *ManipuCommand          `yaml:"entrypoint,omitempty"`
	Cmd        *ManipuCommand          `yaml:"cmd,omitempty"`
	Variants   map[string]ManipuConfig `yaml:"variants,omitempty"`
}

// DefaultConfig returns the top-level defaults assembled as a
// `ManipuConfig`. The returned value is `IsZero()` when no defaults
// are set.
func (m Manipulation) DefaultConfig() ManipuConfig {
	// Default layers are only the layers from ManipuLayers that have
	// an empty rule — legacy rule-bearing layers become variants.
	defaultLayers := make([]ManipuLayer, 0, len(m.ManipuLayers))
	for _, lyr := range m.ManipuLayers {
		if lyr.Rule == "" {
			defaultLayers = append(defaultLayers, lyr)
		}
	}
	return ManipuConfig{
		Env:        m.Env,
		Workdir:    m.Workdir,
		User:       m.User,
		Volumes:    m.Volumes,
		Entrypoint: m.Entrypoint,
		Cmd:        m.Cmd,
		Layers:     defaultLayers,
	}
}

// hasNewTopLevel reports whether the manipulation has any of the new
// defaults fields set (Env, Workdir, User, Volumes, Entrypoint, Cmd)
// or a non-empty Variants map. Layers without a rule are treated as
// legacy-compatible and do NOT count as new-form on their own.
func (m Manipulation) hasNewTopLevel() bool {
	if len(m.Env) != 0 || m.Workdir != "" || m.User != "" || len(m.Volumes) != 0 {
		return true
	}
	if m.Entrypoint != nil && !m.Entrypoint.IsZero() {
		return true
	}
	if m.Cmd != nil && !m.Cmd.IsZero() {
		return true
	}
	if len(m.Variants) != 0 {
		return true
	}
	return false
}

// hasLegacyRuleLayers reports whether any layer entry carries a legacy
// per-layer `rule:` value.
func (m Manipulation) hasLegacyRuleLayers() bool {
	for _, lyr := range m.ManipuLayers {
		if lyr.Rule != "" {
			return true
		}
	}
	return false
}

// NormalizedVariants returns the variants map that downstream consumers
// should persist to the graph. It rewrites the legacy flat form into a
// map keyed by each layer's rule. Call after validate().
func (m Manipulation) NormalizedVariants() map[string]ManipuConfig {
	// New form: return Variants as-is (possibly empty).
	if m.hasNewTopLevel() {
		out := make(map[string]ManipuConfig, len(m.Variants))
		for k, v := range m.Variants {
			out[k] = v
		}
		return out
	}
	// Legacy form: one variant per rule-bearing layer.
	out := map[string]ManipuConfig{}
	for _, lyr := range m.ManipuLayers {
		if lyr.Rule == "" {
			continue
		}
		cfg := out[lyr.Rule]
		cfg.Layers = append(cfg.Layers, lyr)
		out[lyr.Rule] = cfg
	}
	return out
}

// validate returns a non-nil error when the YAML form is inconsistent.
// It also validates variant keys against the expr parser.
func (m Manipulation) validate(yamlFile string) error {
	legacy := m.hasLegacyRuleLayers()
	newForm := m.hasNewTopLevel()
	if legacy && newForm {
		return fmt.Errorf(
			"manipulation %q: mixed form not allowed — per-layer `rule:` found alongside new top-level fields (env/workdir/user/volumes/entrypoint/cmd/variants)",
			yamlFile,
		)
	}
	// Validate variant keys and embedded ManipuCommand ops.
	for rule, cfg := range m.Variants {
		if _, err := utils.ExprGetLabels(rule); err != nil {
			return fmt.Errorf("manipulation %q: variant rule %q is not parseable: %v", yamlFile, rule, err)
		}
		if err := cfg.validateCommands(yamlFile, "variants."+rule); err != nil {
			return err
		}
	}
	// Validate top-level command ops.
	defCfg := ManipuConfig{
		Entrypoint: m.Entrypoint,
		Cmd:        m.Cmd,
	}
	return defCfg.validateCommands(yamlFile, "<top-level>")
}

// validateCommands ensures any entrypoint/cmd op is one of the allowed
// string values. Empty or nil commands are skipped.
func (c ManipuConfig) validateCommands(yamlFile, scope string) error {
	checkOp := func(fieldName string, cmd *ManipuCommand) error {
		if cmd == nil || cmd.IsZero() {
			return nil
		}
		if !validManipuCmdOp(cmd.Op) {
			return fmt.Errorf(
				"manipulation %q: %s.%s.op %q is invalid (must be one of replace|prepend|append)",
				yamlFile, scope, fieldName, cmd.Op,
			)
		}
		return nil
	}
	if err := checkOp("entrypoint", c.Entrypoint); err != nil {
		return err
	}
	return checkOp("cmd", c.Cmd)
}

// validManipuCmdOp mirrors the types.ManipuCmdOp constants. We duplicate
// the allowed set here to avoid pulling the types package into the
// config package and creating a cycle.
func validManipuCmdOp(op string) bool {
	switch op {
	case "replace", "prepend", "append":
		return true
	default:
		return false
	}
}

func FindManipulations(ctx context.Context, cacheDir string) (mps []Manipulation, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "config",
		"func":    "FindManipulations",
		"dir":     cacheDir,
	})
	defer span.End()
	mps = []Manipulation{}
	files, err := FindYamlFiles(cacheDir)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "Error finding yaml files in %s", cacheDir)
		return
	}
	// Parse each filter file
	for _, ffile := range files {
		mp, e := LoadManipulation(ctx, ffile)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "error parsing rule file %s", ffile)
			return
		}
		mps = append(mps, mp)
	}
	return
}

func LoadManipulation(ctx context.Context, yamlfile string) (mp Manipulation, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "config",
		"func":    "LoadManipulation",
		"file":    yamlfile,
	})
	defer span.End()
	_ = ctx
	re := regexp.MustCompile(`.*/manipulations` + fileRegex)
	var label string
	if m, e := utils.ReSubMatchMap(re, yamlfile); e != nil {
		err = utils.ErrorHandler(l, span, e, "Error parsing file %s", yamlfile)
		return
	} else {
		name := m["dir"]
		fname := m["file"]
		label = fmt.Sprintf("%s#%s", name, fname)
	}
	yamlContent, err := ioutil.ReadFile(yamlfile)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "Cannot load manipulation config file")
		return Manipulation{}, err
	}
	utils.Debug(l, span, "attempting readManipulationConfig()")
	mp, err = readManipulationConfig(yamlContent)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "Cannot read manipulation config file")
		return Manipulation{}, err
	}

	if mp.Label != "" && mp.Label != label {
		err = utils.NewError(l, span, "Manipulation label '%s' does not match directory and file name '%s'; please consider removing it, so ot will be derived by path/filename", mp.Label, label)
		return
	}
	mp.Label = label
	if err = mp.validate(yamlfile); err != nil {
		return Manipulation{}, err
	}
	if mp.hasLegacyRuleLayers() {
		logrus.Warnf(
			"manipulation %q uses the deprecated flat `layers: [{image, rule}]` form; "+
				"please migrate to the defaults + variants schema (see docs)",
			yamlfile,
		)
	}
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
