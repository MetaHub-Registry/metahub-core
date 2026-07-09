package neo

import (
	"context"
	"encoding/json"
	"regexp"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgModelDetail = "neo/model_detail.go"
	// ConfigLabelPrefix is the label prefix of a config manipulation
	// (config/<name>) — the per-profile runtime config oras-model-config-injection
	// pushes alongside the model.
	ConfigLabelPrefix = "config/"
	// modelConfigDirEnv is the env var the base image reads the injected config
	// dir from; metahub-admin repackage sets it on the config manipulation.
	modelConfigDirEnv = "QNIB_MODEL_CONFIG_DIR"
)

// modelNameSafe guards the model name that is inlined into the Cypher query
// (there is no query-parameter API here). Under model-centric-pull the org is
// kept, so names are org-qualified path refs like `qnib/minicpm5` (bare
// single-segment refs like `e2e-gguf43` or `gpt:3.4` still pass). The `/` is
// safe to allow — the guard's job is to block the chars that could break out of
// the inlined string literal (`"`, `\`, whitespace), which stay excluded.
// Mirrors runtimeNameSafe (model_runtime.go), which already allows `/`.
var modelNameSafe = regexp.MustCompile(`^[A-Za-z0-9._:/-]+$`)

// ModelDetailGet returns the card payload for a single model (`model/<name>`):
// every quantization grouped under it (layer image + size), the per-profile
// runtime configs (`config/<name>`), the profiles it is served under, and the
// source ORAS artifact. found is false when no `model/<name>` manipulation
// exists (→ the handler answers 404); an invalid name is likewise not-found.
func (n Neo) ModelDetailGet(ctx context.Context, name string) (detail types.ModelDetail, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgModelDetail,
		"func":    "ModelDetailGet",
		"model":   name,
	})
	defer span.End()
	_ = ctx

	if !modelNameSafe.MatchString(name) {
		utils.Debug(l, span, "rejecting unsafe model name")
		return detail, false, nil
	}
	label := ModelLabelPrefix + name

	// Query A — the model manipulation (existence gate), its profiles, its
	// quantizations (model-layer image + tar-layer size), and its source
	// artifact via the OCI subject reference.
	qa := `MATCH (mp:Manipulation {label: "` + label + `"})`
	qa += ` OPTIONAL MATCH (prf:Profile)-[:USES]->(mp)`
	// Quant layers via both the flat (HAS) and multi-quant variant
	// (HAS_VARIANT→ManipuConfig→HAS_LAYER) paths — post-B1 models are variant-form.
	qa += ` OPTIONAL MATCH (mp)-[:HAS|HAS_DEFAULT|HAS_VARIANT]->(mn)-[:UPSTREAM|HAS_LAYER*1..2]->(up:Upstream)`
	qa += ` OPTIONAL MATCH (up)<-[:ORIGINAL]-(:Manifest)-[:HAS_LAYER]->(lb:Blob)`
	qa += ` WHERE lb.mediaType = "` + types.ModelLayerMediaType + `"`
	qa += ` OPTIONAL MATCH (up)<-[:ORIGINAL]-(:Manifest)-[:SUBJECT_OF]->(mdSrc:Manifest)`
	qa += ` OPTIONAL MATCH (srcRepo:Repo)<-[:OF_REPO]-(:Slot)-[:HAS_PUSHED]->(mdSrc)`
	qa += ` RETURN COALESCE(prf.label, "") AS profile,`
	qa += ` COALESCE(up.repo, "") AS repo,`
	qa += ` COALESCE(up.tag, "") AS tag,`
	qa += ` COALESCE(up.digest, "") AS digest,`
	qa += ` COALESCE(lb.size, 0) AS size,`
	qa += ` COALESCE(srcRepo.repo, "") AS srcRepo,`
	qa += ` COALESCE(mdSrc.digest, "") AS srcDigest`
	utils.Debug(l, span, qa)

	res, err := n.Query(qa)
	if err != nil {
		return detail, false, utils.ErrorHandler(l, span, err, "while executing model query")
	}
	if len(res) == 0 {
		return detail, false, nil // no model/<name> manipulation → not found
	}

	detail.Name = name
	detail.Label = label
	detail.Profiles = make([]string, 0)
	detail.Quantizations = make([]types.ModelImageRef, 0)
	detail.Configs = make([]types.ModelProfileConfig, 0)
	accumulateModelDetailRows(res, &detail)

	// Query B — the per-profile runtime configs (config/<name> manipulation):
	// which profile has a config, the config image, and the in-image config dir
	// the base image reads (QNIB_MODEL_CONFIG_DIR).
	configs, err := n.modelConfigsGet(ctx, name)
	if err != nil {
		return detail, false, utils.ErrorHandler(l, span, err, "while executing config query")
	}
	detail.Configs = configs

	// Compositions — auto-derived from referrers(model): config artefacts
	// (subject→a model-layer quant) plus their {profile,runtime,quant} annotations.
	compositions, err := n.modelCompositionsGet(ctx, name)
	if err != nil {
		return detail, false, utils.ErrorHandler(l, span, err, "while executing composition query")
	}
	detail.Compositions = compositions
	return detail, true, nil
}

// accumulateModelDetailRows folds Query A rows into detail: dedup'd profiles,
// dedup'd quantizations (with running TotalSize), and the first source artefact.
func accumulateModelDetailRows(res []types.Result, detail *types.ModelDetail) {
	seenProfile := make(map[string]bool)
	seenImage := make(map[string]bool)
	for _, r := range res {
		if profile, _ := r.GetStr("profile"); profile != "" && !seenProfile[profile] {
			seenProfile[profile] = true
			detail.Profiles = append(detail.Profiles, profile)
		}
		if repo, _ := r.GetStr("repo"); repo != "" {
			tag, _ := r.GetStr("tag")
			dig, _ := r.GetStr("digest")
			size, _ := r.GetInt64("size")
			imgKey := repo + "|" + tag + "|" + dig
			if !seenImage[imgKey] {
				seenImage[imgKey] = true
				detail.Quantizations = append(detail.Quantizations,
					types.ModelImageRef{Repo: repo, Tag: tag, Digest: dig, Size: size})
				detail.TotalSize += size
			}
		}
		if detail.SourceArtifact == nil {
			if srcDigest, _ := r.GetStr("srcDigest"); srcDigest != "" {
				srcRepo, _ := r.GetStr("srcRepo")
				detail.SourceArtifact = &types.ModelSourceArtifact{Repo: srcRepo, Digest: srcDigest}
			}
		}
	}
}

// modelCompositionsGet assembles the model's compositions from its referrers: a
// config artefact whose OCI subject is one of the model's quant manifests, read
// back with its composition annotations (profile/runtime/quant) and the config
// image it repackages into. One ModelComposition per config artefact.
func (n Neo) modelCompositionsGet(ctx context.Context, name string) (comps []types.ModelComposition, err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgModelDetail,
		"func":    "modelCompositionsGet",
		"model":   name,
	})
	defer span.End()
	comps = make([]types.ModelComposition, 0)

	label := ModelLabelPrefix + name
	q := `MATCH (mp:Manipulation {label: "` + label + `"})-[:HAS|HAS_DEFAULT|HAS_VARIANT]->(mn)-[:UPSTREAM|HAS_LAYER*1..2]->(up:Upstream)<-[:ORIGINAL]-(modelMd:Manifest)`
	q += ` MATCH (cfgMd:Manifest)-[:SUBJECT_OF]->(modelMd)`
	q += ` OPTIONAL MATCH (cfgMd)-[:ORIGINAL]->(cfgUp:Upstream)`
	q += ` OPTIONAL MATCH (cfgMd)-[:HAS_ANNOTATION]->(a:Annotation)`
	q += ` RETURN cfgMd.digest AS cfgDigest,`
	q += ` COALESCE(up.tag, "") AS modelQuant,`
	q += ` COALESCE(cfgUp.repo, "") AS cfgRepo,`
	q += ` COALESCE(cfgUp.tag, "") AS cfgTag,`
	q += ` COALESCE(a.key, "") AS annKey,`
	q += ` COALESCE(a.value, "") AS annValue`
	utils.Debug(l, span, q)

	res, err := n.Query(q)
	if err != nil {
		return comps, err
	}
	comps = collectCompositions(res)
	n.resolveCompositionRuntimeImages(ctx, comps)
	return comps, nil
}

// collectCompositions folds composition-query rows into one ModelComposition per
// config-artefact digest (first-seen order), applying each row's annotation.
func collectCompositions(res []types.Result) []types.ModelComposition {
	byDigest := make(map[string]*types.ModelComposition)
	order := make([]string, 0)
	for _, r := range res {
		cfgDigest, _ := r.GetStr("cfgDigest")
		if cfgDigest == "" {
			continue
		}
		c, ok := byDigest[cfgDigest]
		if !ok {
			modelQuant, _ := r.GetStr("modelQuant")
			cfgRepo, _ := r.GetStr("cfgRepo")
			cfgTag, _ := r.GetStr("cfgTag")
			c = &types.ModelComposition{
				Quant:        modelQuant, // default to the subject quant; overridden by the quant annotation below
				ConfigRepo:   cfgRepo,
				ConfigTag:    cfgTag,
				ConfigDigest: cfgDigest,
			}
			byDigest[cfgDigest] = c
			order = append(order, cfgDigest)
		}
		applyCompositionAnnotation(c, r)
	}
	comps := make([]types.ModelComposition, 0, len(order))
	for _, d := range order {
		comps = append(comps, *byDigest[d])
	}
	return comps
}

// applyCompositionAnnotation applies one {profile,runtime,quant} annotation row.
func applyCompositionAnnotation(c *types.ModelComposition, r types.Result) {
	switch key, _ := r.GetStr("annKey"); key {
	case types.ModelProfileAnnotation:
		c.Profile, _ = r.GetStr("annValue")
	case types.ModelRuntimeAnnotation:
		c.Runtime, _ = r.GetStr("annValue")
	case types.ModelQuantAnnotation:
		if v, _ := r.GetStr("annValue"); v != "" {
			c.Quant = v
		}
	}
}

// resolveCompositionRuntimeImages fills each composition's RuntimeImage from the
// runtime registry (best-effort; an unregistered runtime stays image-less).
func (n Neo) resolveCompositionRuntimeImages(ctx context.Context, comps []types.ModelComposition) {
	if len(comps) == 0 {
		return
	}
	rts, e := n.RuntimesGetAll(ctx)
	if e != nil {
		return
	}
	byName := make(map[string]string, len(rts))
	for _, rt := range rts {
		byName[rt.Name] = rt.Image
	}
	for i := range comps {
		if img, ok := byName[comps[i].Runtime]; ok {
			comps[i].RuntimeImage = img
		}
	}
}

// modelConfigsGet reads the config/<name> manipulation's per-profile configs.
func (n Neo) modelConfigsGet(ctx context.Context, name string) (configs []types.ModelProfileConfig, err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgModelDetail,
		"func":    "modelConfigsGet",
		"model":   name,
	})
	defer span.End()
	configs = make([]types.ModelProfileConfig, 0)

	label := ConfigLabelPrefix + name
	q := `MATCH (mp:Manipulation {label: "` + label + `"})`
	q += ` OPTIONAL MATCH (prf:Profile)-[:USES]->(mp)`
	q += ` OPTIONAL MATCH (mp)-[:HAS_DEFAULT|HAS_VARIANT]->(mc:ManipuConfig)`
	q += ` OPTIONAL MATCH (mc)-[:HAS_LAYER]->(:ManipuLayer)-[:UPSTREAM]->(cup:Upstream)`
	q += ` RETURN COALESCE(prf.label, "") AS profile,`
	q += ` COALESCE(cup.repo, "") AS repo,`
	q += ` COALESCE(cup.tag, "") AS tag,`
	q += ` COALESCE(cup.digest, "") AS digest,`
	q += ` COALESCE(mc.env, "") AS env`
	utils.Debug(l, span, q)

	res, err := n.Query(q)
	if err != nil {
		return configs, err
	}
	seen := make(map[string]bool)
	for _, r := range res {
		profile, _ := r.GetStr("profile")
		if profile == "" || seen[profile] {
			continue
		}
		seen[profile] = true
		repo, _ := r.GetStr("repo")
		tag, _ := r.GetStr("tag")
		dig, _ := r.GetStr("digest")
		env, _ := r.GetStr("env")
		configs = append(configs, types.ModelProfileConfig{
			Profile:   profile,
			Repo:      repo,
			Tag:       tag,
			Digest:    dig,
			ConfigDir: configDirFromEnv(env),
		})
	}
	return configs, nil
}

// configDirFromEnv pulls QNIB_MODEL_CONFIG_DIR out of a ManipuConfig.env JSON
// object (e.g. `{"QNIB_MODEL_CONFIG_DIR":"/model/e2e-gguf43/config"}`). Returns
// "" when env is empty or unparseable.
func configDirFromEnv(env string) string {
	if env == "" {
		return ""
	}
	var m map[string]string
	if json.Unmarshal([]byte(env), &m) != nil {
		return ""
	}
	return m[modelConfigDirEnv]
}
