package neo

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgOrasName = "neo/oras.go"
)

// containerImageConfigFilter returns a Cypher IN list for the container image config media type allowlist.
// Uses cfg.mediaType property (always the full MIME string) to correctly identify container images
// regardless of how the relationship type was encoded (full MIME vs ShortenMediaType form).
func containerImageConfigFilter() string {
	return `["application/vnd.oci.image.config.v1+json", "application/vnd.docker.container.image.v1+json"]`
}

// OrasArtifactGetByRepoLike returns OrasListing entries for all ORAS artifact repos
// whose path matches the given prefix (regex: ^<repo>.*). Only manifests whose config
// relationship type is NOT in the container image allowlist are returned.
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

	q := fmt.Sprintf(`MATCH (c:Collection)-[:HAS_TAG]->(t) MATCH (c)-[:UPSTREAM]->(m)-[r]-(md:ManifestData) MATCH (md)-[cfgRel]->(cfg:Blob) WHERE cfg.type = "config" AND c.repo =~ "%s" AND NOT cfg.mediaType IN %s RETURN c.repo AS repo, cfg.mediaType AS artifactType, count(md) AS artifactCount`,
		repoPattern, containerImageConfigFilter())

	utils.Debug(l, span, q)
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

	q := fmt.Sprintf(`MATCH (c:Collection)-[:HAS_TAG]->(t) MATCH (c)-[:UPSTREAM]->(m)-[r]-(md:ManifestData) MATCH (md)-[cfgRel]->(cfg:Blob) WHERE cfg.type = "config" AND c.repo = "%s" AND cfg.mediaType = "%s" RETURN id(md) AS manifestDataID, md.digest AS manifestDataDigest, md.mediaType AS manifestDataMediaType, c.repo AS repo, cfg.mediaType AS artifactType LIMIT 1`,
		repo, artifactType)

	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		return artifact, err
	}
	if !found {
		return artifact, fmt.Errorf("ORAS artifact not found: repo=%s artifactType=%s", repo, artifactType)
	}

	artifact, err = orasArtifactFromResult(res)
	return
}

// OrasArtifactGetByID returns a single OrasArtifact matching the given Neo4j ManifestData node ID.
func (n Neo) OrasArtifactGetByID(ctx context.Context, manifestID int64) (artifact types.OrasArtifact, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgOrasName,
		"func":       "OrasArtifactGetByID",
		"manifestID": manifestID,
	})
	defer span.End()
	_ = ctx

	q := fmt.Sprintf(`MATCH (c:Collection)-[:HAS_TAG]->(t) MATCH (c)-[:UPSTREAM]->(m)-[r]-(md:ManifestData) MATCH (md)-[cfgRel]->(cfg:Blob) WHERE cfg.type = "config" AND id(md) = %d AND NOT cfg.mediaType IN %s RETURN id(md) AS manifestDataID, md.digest AS manifestDataDigest, md.mediaType AS manifestDataMediaType, c.repo AS repo, cfg.mediaType AS artifactType LIMIT 1`,
		manifestID, containerImageConfigFilter())

	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		return artifact, err
	}
	if !found {
		return artifact, fmt.Errorf("ORAS artifact not found: id=%d", manifestID)
	}

	artifact, err = orasArtifactFromResult(res)
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
		err = nil
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
		a.Blobs = []types.ManifestData{}
	}
	return
}
