package neo

import (
	"context"
	"fmt"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/sirupsen/logrus"
)

const (
	orasEnrichWarn = "OrasArtifactEnrich failed (non-fatal): %v"
	pkgOrasName    = "neo/oras.go"
)

// containerImageConfigFilter returns a Cypher IN list for the container image config media type allowlist.
// Uses cfg.mediaType property (always the full MIME string) to correctly identify container images
// regardless of how the relationship type was encoded (full MIME vs ShortenMediaType form).
func containerImageConfigFilter() string {
	return `["application/vnd.oci.image.config.v1+json", "application/vnd.docker.container.image.v1+json"]`
}

// OrasArtifactGetByRepoLike returns OrasListing entries for all ORAS artifact repos
// whose path matches the given prefix (regex: ^<repo>.*). Walks the Slot-centred model:
// Repo <-[:OF_REPO]- Slot -[:HAS_PUSHED]-> Manifest -[:USES]-> Blob.
// Only manifests whose config blob mediaType is NOT in the container image allowlist are returned.
func (n Neo) OrasArtifactGetByRepoLike(ctx context.Context, repo string) (listings []types.OrasListing, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgOrasName,
		"func":    "OrasArtifactGetByRepoLike",
		"repo":    repo,
	})
	defer span.End()
	_ = ctx

	repoPattern := repo
	if repoPattern == "" {
		repoPattern = ".*"
	} else {
		repoPattern = fmt.Sprintf("^%s.*", repoPattern)
	}

	q := fmt.Sprintf(`MATCH (c:Repo)<-[:OF_REPO]-(s:Slot)-[:HAS_PUSHED]->(md:Manifest)-[:USES]->(cfg:Blob) WHERE cfg.type = "config" AND c.repo =~ "%s" AND NOT cfg.mediaType IN %s RETURN c.repo AS repo, COALESCE(md.artifactType, cfg.mediaType) AS artifactType, count(DISTINCT md) AS artifactCount`,
		repoPattern, containerImageConfigFilter())

	utils.Debug(l, span, "%s", q)
	res, err := n.Query(q)
	if err != nil {
		utils.ErrorHandler(l, span, err, "while executing Query")
		return listings, err
	}

	for _, r := range res {
		repoVal, e := r.GetStr("repo")
		if e != nil {
			utils.ErrorHandler(l, span, e, "while reading repo")
			return listings, e
		}
		artType, e := r.GetStr("artifactType")
		if e != nil {
			utils.ErrorHandler(l, span, e, "while reading artifactType")
			return listings, e
		}
		count, e := r.GetInt64("artifactCount")
		if e != nil {
			utils.ErrorHandler(l, span, e, "while reading artifactCount")
			return listings, e
		}
		listings = append(listings, types.OrasListing{
			Repo:          repoVal,
			ArtifactType:  artType,
			ArtifactCount: count,
		})
	}
	logrus.Debugf("OrasArtifactGetByRepoLike: found %d listings", len(listings))
	return
}

// OrasArtifactGetByName returns a single OrasArtifact matching the given repo and artifactType.
func (n Neo) OrasArtifactGetByName(ctx context.Context, repo, artifactType string) (artifact types.OrasArtifact, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":      pkgOrasName,
		"func":         "OrasArtifactGetByName",
		"repo":         repo,
		"artifactType": artifactType,
	})
	defer span.End()
	_ = ctx

	q := fmt.Sprintf(`MATCH (c:Repo)<-[:OF_REPO]-(s:Slot)-[:HAS_PUSHED]->(md:Manifest)-[:USES]->(cfg:Blob) WHERE cfg.type = "config" AND c.repo = "%s" AND COALESCE(md.artifactType, cfg.mediaType) = "%s" RETURN id(md) AS manifestDataID, md.digest AS manifestDataDigest, md.mediaType AS manifestDataMediaType, c.repo AS repo, COALESCE(md.artifactType, cfg.mediaType) AS artifactType LIMIT 1`,
		repo, artifactType)

	utils.Debug(l, span, "%s", q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		return artifact, err
	}
	if !found {
		return artifact, fmt.Errorf("ORAS artifact not found: repo=%s artifactType=%s", repo, artifactType)
	}

	artifact, err = orasArtifactFromResult(res)
	if err != nil {
		return artifact, err
	}
	if err = n.OrasArtifactEnrich(ctx, artifact.ID, &artifact); err != nil {
		utils.Warn(l, span, orasEnrichWarn, err)
		err = nil // enrichment failure is non-fatal — return basic artifact
	}
	return
}

// OrasArtifactGetByDigest returns a single OrasArtifact matching the given
// repo and manifest digest. Public callers address an artifact by repo +
// digest; the element ID stays inside the query.
func (n Neo) OrasArtifactGetByDigest(ctx context.Context, repo, dig string) (artifact types.OrasArtifact, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgOrasName,
		"func":    "OrasArtifactGetByDigest",
		"repo":    repo,
		"digest":  utils.ShortenDigest(dig),
	})
	defer span.End()
	_ = ctx

	q := fmt.Sprintf(`MATCH (c:Repo)<-[:OF_REPO]-(s:Slot)-[:HAS_PUSHED]->(md:Manifest)-[:USES]->(cfg:Blob) WHERE cfg.type = "config" AND c.repo = "%s" AND md.digest = "%s" AND NOT cfg.mediaType IN %s RETURN id(md) AS manifestDataID, md.digest AS manifestDataDigest, md.mediaType AS manifestDataMediaType, c.repo AS repo, COALESCE(md.artifactType, cfg.mediaType) AS artifactType LIMIT 1`,
		repo, dig, containerImageConfigFilter())

	utils.Debug(l, span, "%s", q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		return artifact, err
	}
	if !found {
		return artifact, fmt.Errorf("ORAS artifact not found: repo=%s digest=%s", repo, dig)
	}

	artifact, err = orasArtifactFromResult(res)
	if err != nil {
		return artifact, err
	}
	if err = n.OrasArtifactEnrich(ctx, artifact.ID, &artifact); err != nil {
		utils.Warn(l, span, orasEnrichWarn, err)
		err = nil
	}
	return
}

// OrasArtifactGetByID returns a single OrasArtifact matching the given Neo4j Manifest node ID.
func (n Neo) OrasArtifactGetByID(ctx context.Context, manifestID int64) (artifact types.OrasArtifact, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgOrasName,
		"func":       "OrasArtifactGetByID",
		"manifestID": manifestID,
	})
	defer span.End()
	_ = ctx

	q := fmt.Sprintf(`MATCH (c:Repo)<-[:OF_REPO]-(s:Slot)-[:HAS_PUSHED]->(md:Manifest)-[:USES]->(cfg:Blob) WHERE cfg.type = "config" AND id(md) = %d AND NOT cfg.mediaType IN %s RETURN id(md) AS manifestDataID, md.digest AS manifestDataDigest, md.mediaType AS manifestDataMediaType, c.repo AS repo, COALESCE(md.artifactType, cfg.mediaType) AS artifactType LIMIT 1`,
		manifestID, containerImageConfigFilter())

	utils.Debug(l, span, "%s", q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		return artifact, err
	}
	if !found {
		return artifact, fmt.Errorf("ORAS artifact not found: id=%d", manifestID)
	}

	artifact, err = orasArtifactFromResult(res)
	if err != nil {
		return artifact, err
	}
	if err = n.OrasArtifactEnrich(ctx, artifact.ID, &artifact); err != nil {
		utils.Warn(l, span, orasEnrichWarn, err)
		err = nil
	}
	return
}

// orasArtifactFromResult maps a types.Result row into an OrasArtifact.
func orasArtifactFromResult(res types.Result) (a types.OrasArtifact, err error) {
	a.ID, err = res.GetInt64("manifestDataID")
	if err != nil {
		return a, fmt.Errorf("orasArtifactFromResult: manifestDataID: %w", err)
	}
	a.Digest, err = res.GetStr("manifestDataDigest")
	if err != nil {
		return a, fmt.Errorf("orasArtifactFromResult: manifestDataDigest: %w", err)
	}
	a.MediaType, err = res.GetStr("manifestDataMediaType")
	if err != nil {
		// mediaType may be empty for some manifests — not fatal
		a.MediaType = ""
	}
	a.Repo, err = res.GetStr("repo")
	if err != nil {
		return a, fmt.Errorf("orasArtifactFromResult: repo: %w", err)
	}
	a.ArtifactType, err = res.GetStr("artifactType")
	if err != nil {
		return a, fmt.Errorf("orasArtifactFromResult: artifactType: %w", err)
	}
	if a.Annotations == nil {
		a.Annotations = map[string]string{}
	}
	if a.Blobs == nil {
		a.Blobs = []types.OrasBlob{}
	}
	return
}

// OrasArtifactEnrich populates the Blobs and Annotations fields of an
// OrasArtifact by querying existing graph data. No graph writes occur.
func (n Neo) OrasArtifactEnrich(ctx context.Context, manifestID int64, artifact *types.OrasArtifact) error {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgOrasName,
		"func":       "OrasArtifactEnrich",
		"manifestID": manifestID,
	})
	defer span.End()

	// 1. Fetch manifest-level annotations
	qAnnotations := fmt.Sprintf(
		`MATCH (md:Manifest)-[:HAS_ANNOTATION]->(a:Annotation) WHERE id(md) = %d RETURN a.key AS key, a.value AS value`,
		manifestID,
	)
	utils.Debug(l, span, "%s", qAnnotations)
	annoResults, err := n.Query(qAnnotations)
	if err != nil {
		return fmt.Errorf("OrasArtifactEnrich: manifest annotations: %w", err)
	}
	if artifact.Annotations == nil {
		artifact.Annotations = map[string]string{}
	}
	for _, r := range annoResults {
		key, _ := r.GetStr("key")
		value, _ := r.GetStr("value")
		if key != "" {
			artifact.Annotations[key] = value
		}
	}

	// 2. Fetch layer blobs
	qLayers := fmt.Sprintf(
		`MATCH (md:Manifest)-[r:HAS_LAYER]->(b:Blob) WHERE id(md) = %d AND b.type = "layer" RETURN id(b) AS blobID, b.digest AS digest, b.mediaType AS mediaType, b.size AS size ORDER BY r.order`,
		manifestID,
	)
	utils.Debug(l, span, "%s", qLayers)
	layerResults, err := n.Query(qLayers)
	if err != nil {
		return fmt.Errorf("OrasArtifactEnrich: layers: %w", err)
	}

	type blobEntry struct {
		idx    int
		blobID int64
	}
	var blobEntries []blobEntry
	artifact.Blobs = make([]types.OrasBlob, 0, len(layerResults))
	for _, r := range layerResults {
		blob := types.OrasBlob{
			Annotations: map[string]string{},
		}
		blob.Digest, _ = r.GetStr("digest")
		blob.MediaType, _ = r.GetStr("mediaType")
		blob.Size, _ = r.GetInt64("size")
		blobID, _ := r.GetInt64("blobID")
		blobEntries = append(blobEntries, blobEntry{idx: len(artifact.Blobs), blobID: blobID})
		artifact.Blobs = append(artifact.Blobs, blob)
	}

	// 3. Fetch annotations for all layer blobs in one query
	if len(blobEntries) > 0 {
		ids := ""
		for i, be := range blobEntries {
			if i > 0 {
				ids += ", "
			}
			ids += fmt.Sprintf("%d", be.blobID)
		}
		qBlobAnnos := fmt.Sprintf(
			`MATCH (b:Blob)-[:HAS_ANNOTATION]->(a:Annotation) WHERE id(b) IN [%s] RETURN id(b) AS blobID, a.key AS key, a.value AS value`,
			ids,
		)
		utils.Debug(l, span, "%s", qBlobAnnos)
		blobAnnoResults, err := n.Query(qBlobAnnos)
		if err != nil {
			return fmt.Errorf("OrasArtifactEnrich: blob annotations: %w", err)
		}
		idxMap := map[int64]int{}
		for _, be := range blobEntries {
			idxMap[be.blobID] = be.idx
		}
		for _, r := range blobAnnoResults {
			blobID, _ := r.GetInt64("blobID")
			key, _ := r.GetStr("key")
			value, _ := r.GetStr("value")
			if idx, ok := idxMap[blobID]; ok && key != "" {
				artifact.Blobs[idx].Annotations[key] = value
				if key == "org.opencontainers.image.title" {
					artifact.Blobs[idx].Title = value
				}
			}
		}
	}

	utils.Debug(l, span, "Enriched artifact with %d annotations and %d blobs", len(artifact.Annotations), len(artifact.Blobs))
	return nil
}
