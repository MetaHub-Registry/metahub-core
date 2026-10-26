package neo

import (
	"context"
	"strings"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgModelListing = "neo/model_listing.go"
	// ModelLabelPrefix is the label prefix of a model manipulation (model/<name>).
	// Repackaging an ORAS model artifact creates a `model/<name>` manipulation
	// linked to the profile the model is pulled under.
	ModelLabelPrefix = "model/"
)

// ModelListingGetAll returns one ModelListing per `model/<name>` manipulation:
// the model name, the profiles that inject it (Profile-[:USES]->Manipulation),
// and its repackaged model-layer image variants
// (Manipulation-[:HAS]->ManipuLayer-[:UPSTREAM]->Upstream). The query returns one
// row per (model, profile, image) combination which is aggregated by label in Go
// (the result helpers are scalar-only, so collecting maps in Cypher is avoided).
func (n Neo) ModelListingGetAll(ctx context.Context) (listings []types.ModelListing, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgModelListing,
		"func":    "ModelListingGetAll",
	})
	defer span.End()
	_ = ctx

	q := `MATCH (mp:Manipulation) WHERE mp.label STARTS WITH "` + ModelLabelPrefix + `"`
	q += ` OPTIONAL MATCH (prf:Profile)-[:USES]->(mp)`
	q += ` OPTIONAL MATCH (mp)-[:HAS|HAS_DEFAULT|HAS_VARIANT]->(mn)-[:UPSTREAM|HAS_LAYER*1..2]->(up:Upstream)`
	// Model-layer size: the model-layer image's uncompressed tar layer blob is
	// the model weight; its size is what the catalog reports per quantization.
	q += ` OPTIONAL MATCH (up)<-[:ORIGINAL]-(:Manifest)-[:HAS_LAYER]->(lb:Blob)`
	q += ` WHERE lb.mediaType = "` + types.ModelLayerMediaType + `"`
	// Source ORAS artifact via the model-layer image's OCI subject reference:
	// up <-[:ORIGINAL]- model-layer Manifest -[:SUBJECT_OF]-> source Manifest,
	// whose Repo is reached through the Slot-centred push pattern.
	q += ` OPTIONAL MATCH (up)<-[:ORIGINAL]-(:Manifest)-[:SUBJECT_OF]->(mdSrc:Manifest)`
	q += ` OPTIONAL MATCH (srcRepo:Repo)<-[:OF_REPO]-(:Slot)-[:HAS_PUSHED]->(mdSrc)`
	q += ` RETURN mp.label AS label,`
	q += ` COALESCE(prf.label, "") AS profile,`
	q += ` COALESCE(up.repo, "") AS repo,`
	q += ` COALESCE(up.tag, "") AS tag,`
	q += ` COALESCE(up.digest, "") AS digest,`
	q += ` COALESCE(lb.size, 0) AS size,`
	q += ` COALESCE(srcRepo.repo, "") AS srcRepo,`
	q += ` COALESCE(mdSrc.digest, "") AS srcDigest`
	utils.Debug(l, span, "%s", q)

	res, err := n.Query(q)
	if err != nil {
		return listings, utils.ErrorHandler(l, span, err, "while executing Query")
	}

	listings, err = buildModelListings(res)
	if err != nil {
		return listings, utils.ErrorHandler(l, span, err, "while reading label")
	}
	return listings, nil
}

// buildModelListings folds catalog rows into one ModelListing per model label
// (first-seen order), each with dedup'd profiles/images and its source artefact.
func buildModelListings(res []types.Result) ([]types.ModelListing, error) {
	byLabel := make(map[string]*types.ModelListing)
	order := make([]string, 0)
	seenProfile := make(map[string]map[string]bool)
	seenImage := make(map[string]map[string]bool)
	for _, r := range res {
		label, e := r.GetStr("label")
		if e != nil {
			return nil, e
		}
		ml, ok := byLabel[label]
		if !ok {
			ml = &types.ModelListing{
				Name:     strings.TrimPrefix(label, ModelLabelPrefix),
				Label:    label,
				Profiles: make([]string, 0),
				Images:   make([]types.ModelImageRef, 0),
			}
			byLabel[label] = ml
			order = append(order, label)
			seenProfile[label] = make(map[string]bool)
			seenImage[label] = make(map[string]bool)
		}
		accumulateListingRow(r, ml, seenProfile[label], seenImage[label])
	}
	listings := make([]types.ModelListing, 0, len(order))
	for _, label := range order {
		listings = append(listings, *byLabel[label])
	}
	return listings, nil
}

// accumulateListingRow folds one row's profile / image / source artefact into ml
// (dedup'd via the per-label seen maps).
func accumulateListingRow(r types.Result, ml *types.ModelListing, seenProfile, seenImage map[string]bool) {
	if profile, _ := r.GetStr("profile"); profile != "" && !seenProfile[profile] {
		seenProfile[profile] = true
		ml.Profiles = append(ml.Profiles, profile)
	}
	if repo, _ := r.GetStr("repo"); repo != "" {
		tag, _ := r.GetStr("tag")
		dig, _ := r.GetStr("digest")
		size, _ := r.GetInt64("size")
		imgKey := repo + "|" + tag + "|" + dig
		if !seenImage[imgKey] {
			seenImage[imgKey] = true
			ml.Images = append(ml.Images, types.ModelImageRef{Repo: repo, Tag: tag, Digest: dig, Size: size})
			ml.TotalSize += size
		}
	}
	if ml.SourceArtifact == nil {
		if srcDigest, _ := r.GetStr("srcDigest"); srcDigest != "" {
			srcRepo, _ := r.GetStr("srcRepo")
			ml.SourceArtifact = &types.ModelSourceArtifact{Repo: srcRepo, Digest: srcDigest}
		}
	}
}
