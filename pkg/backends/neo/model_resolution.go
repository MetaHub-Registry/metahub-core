package neo

import (
	"context"
	"regexp"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const pkgModelResolution = "neo/model_resolution.go"

// quantTagSafe guards the quant tag inlined into the variant-upsert query.
var quantTagSafe = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

// ModelQuantVariantUpsert idempotently registers a quantization as a variant of
// the `model/<name>` manipulation: it ensures a `HAS_VARIANT {rule:"quant:<q>"}`
// ManipuConfig whose ManipuLayer links to the already-pushed model-layer image
// (identified by imgDigest). Repeatedly calling it for different quants
// accumulates one variant per quant under a single model (the multi-quant model
// the resolver picks the best fit from). It links to the existing
// Manifest/Upstream (wired by the repackage push) rather than re-fetching, so it
// never hits the registry. Returns the model manipulation's graph id.
func (n Neo) ModelQuantVariantUpsert(ctx context.Context, name, quant, imgDigest string) (mpID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgModelResolution,
		"func":    "ModelQuantVariantUpsert",
		"model":   name,
		"quant":   quant,
	})
	defer span.End()
	_ = ctx

	if !modelNameSafe.MatchString(name) {
		return -1, utils.NewError(l, span, "unsafe model name %q", name)
	}
	if !quantTagSafe.MatchString(quant) {
		return -1, utils.NewError(l, span, "unsafe quant tag %q", quant)
	}
	if imgDigest == "" {
		return -1, utils.NewError(l, span, "empty image digest for model %q quant %q", name, quant)
	}
	label := ModelLabelPrefix + name
	rule := "quant:" + quant

	// MATCH the pushed model-layer image FIRST so a missing manifest writes
	// nothing (no partial manipulation). Then MERGE the model manipulation and
	// add the quant's variant idempotently, linking to the existing image.
	q := `MATCH (img:Manifest {digest: "` + imgDigest + `"})-[:ORIGINAL]->(up:Upstream)`
	// A model-layer manifest can carry more than one ORIGINAL→Upstream edge: the
	// push registers one Upstream keyed on the API's own host, while repackage's
	// UpstreamGetInsert adds another keyed on the client registry URL (same
	// repo:tag:digest, different reg). Collapse to a single (img, up) row here —
	// otherwise the ManipuLayer MERGE below fires once per row and the second
	// insert violates the ManipuLayer (label, rule) UNIQUE constraint, rolling
	// back the whole upsert (silently, since Execute does not surface the
	// commit-time constraint error) so no manipulation ever persists.
	q += ` WITH img, up ORDER BY id(up) LIMIT 1`
	q += ` MERGE (mp:Manipulation {label: "` + label + `"})`
	q += ` WITH mp, img, up`
	q += ` OPTIONAL MATCH (mp)-[:HAS_VARIANT {rule: "` + rule + `"}]->(mc0:ManipuConfig)`
	q += ` FOREACH (_ IN CASE WHEN mc0 IS NULL THEN [1] ELSE [] END |`
	q += `   CREATE (mp)-[:HAS_VARIANT {rule: "` + rule + `"}]->(:ManipuConfig) )`
	q += ` WITH mp, img, up`
	q += ` MATCH (mp)-[:HAS_VARIANT {rule: "` + rule + `"}]->(mc:ManipuConfig)`
	q += ` MERGE (mc)-[:HAS_LAYER]->(mpl:ManipuLayer {label: "` + label + `", rule: "` + rule + `"})`
	q += `   ON CREATE SET mpl.select = ""`
	// Replace (not accumulate) the quant variant's model-layer image link. A
	// re-repackage of the same quant that produced a different manifest digest
	// (non-deterministic assembly, or a re-seed) would otherwise leave a second
	// UPSTREAM/USES edge on this ManipuLayer, so ModelQuantsGetByName — keyed on
	// repo|tag|digest — surfaces the quant twice. A quant variant references
	// exactly one (current) model-layer image, so drop links to any other image.
	q += ` WITH DISTINCT mp, img, up, mpl`
	q += ` OPTIONAL MATCH (mpl)-[oldU:UPSTREAM]->(oldUp:Upstream) WHERE oldUp <> up`
	q += ` DELETE oldU`
	q += ` WITH DISTINCT mp, img, up, mpl`
	q += ` OPTIONAL MATCH (mpl)-[oldS:USES]->(oldImg:Manifest) WHERE oldImg <> img`
	q += ` DELETE oldS`
	q += ` WITH DISTINCT mp, img, up, mpl`
	q += ` MERGE (mpl)-[:UPSTREAM]->(up)`
	q += ` MERGE (mpl)-[:USES]->(img)`
	q += ` RETURN id(mp) AS mpID`
	utils.Debug(l, span, q)

	res, err := n.Execute(q)
	if err != nil {
		return -1, utils.ErrorHandler(l, span, err, "while executing ModelQuantVariantUpsert")
	}
	mpID, err = res.GetInt64("mpID")
	if err != nil {
		// No RETURN row → the model-layer manifest was not found (push it first).
		return -1, utils.NewError(l, span, "model-layer manifest %q not found for model %q quant %q (push the image first)", imgDigest, name, quant)
	}
	return mpID, nil
}

// ModelQuantsGetByName returns the quantization variants of a model — one
// ModelImageRef per `models/<name>-layer:<quant>` image (Tag = quant, Size = the
// model-layer weights), gathered from the `model/<name>` manipulation via both
// the flat (`HAS`) and the variant (`HAS_DEFAULT|HAS_VARIANT`) layer paths so it
// works before and after the multi-quant-variant repackage. It is the input to
// types.ResolveModelServe — the set the pull-path resolver picks the served quant
// from. Returns an empty slice for an unknown or unsafe name.
func (n Neo) ModelQuantsGetByName(ctx context.Context, name string) (quants []types.ModelImageRef, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgModelResolution,
		"func":    "ModelQuantsGetByName",
		"model":   name,
	})
	defer span.End()
	_ = ctx
	quants = make([]types.ModelImageRef, 0)

	if !modelNameSafe.MatchString(name) {
		return quants, nil
	}
	label := ModelLabelPrefix + name

	q := `MATCH (mp:Manipulation {label: "` + label + `"})`
	q += ` OPTIONAL MATCH (mp)-[:HAS]->(:ManipuLayer)-[:UPSTREAM]->(up1:Upstream)`
	q += ` OPTIONAL MATCH (mp)-[:HAS_DEFAULT|HAS_VARIANT]->(:ManipuConfig)-[:HAS_LAYER]->(:ManipuLayer)-[:UPSTREAM]->(up2:Upstream)`
	q += ` WITH [x IN collect(DISTINCT up1) + collect(DISTINCT up2) WHERE x IS NOT NULL] AS ups`
	q += ` UNWIND ups AS up`
	q += ` MATCH (up)<-[:ORIGINAL]-(:Manifest)-[:HAS_LAYER]->(lb:Blob)`
	q += ` WHERE lb.mediaType = "` + types.ModelLayerMediaType + `"`
	q += ` RETURN DISTINCT COALESCE(up.repo, "") AS repo,`
	q += ` COALESCE(up.tag, "") AS tag,`
	q += ` COALESCE(up.digest, "") AS digest,`
	q += ` COALESCE(lb.size, 0) AS size`
	utils.Debug(l, span, q)

	res, err := n.Query(q)
	if err != nil {
		return quants, utils.ErrorHandler(l, span, err, "while executing ModelQuantsGetByName")
	}
	seen := make(map[string]bool)
	for _, r := range res {
		repo, _ := r.GetStr("repo")
		tag, _ := r.GetStr("tag")
		dig, _ := r.GetStr("digest")
		size, _ := r.GetInt64("size")
		key := repo + "|" + tag + "|" + dig
		if repo == "" || seen[key] {
			continue
		}
		seen[key] = true
		quants = append(quants, types.ModelImageRef{Repo: repo, Tag: tag, Digest: dig, Size: size})
	}
	return quants, nil
}
