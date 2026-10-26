package neo

import (
	"context"
	"fmt"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	neoSubFile = "neo/subject.go"
)

func (neo Neo) LinkSubjectByDigestMediaType(ctx context.Context, refID int64, subDigest, subMediaType string) (err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":      neoSubFile,
		"func":         "LinkSubjectByDigestMediaType",
		"ref":          refID,
		"subDigest":    subDigest,
		"subMediaType": subMediaType,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (r),(s) WHERE id(r) = %d AND s.digest = "%s" AND s.mediaType = "%s"`, refID, subDigest, subMediaType)
	q += ` MERGE (r)-[:SUBJECT_OF]->(s)`
	utils.Debug(l, span, "%s", q)
	_, err = neo.Execute(q)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "while linking Subject")
	}
	return nil
}

func (neo Neo) ReferrersGetBySubjectDigest(ctx context.Context, dig string) (refs types.ReferrersResponse, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": neoSubFile,
		"func":    "ReferrersGetBySubjectDigest",
		"digest":  dig,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (s)<-[:SUBJECT_OF]-(r) WHERE s.digest = "%s" RETURN r.digest AS digest, r.mediaType AS mediaType, labels(r) AS nodeType`, dig)
	utils.Debug(l, span, "%s", q)
	res, err := neo.Query(q)
	if err != nil {
		return refs, utils.ErrorHandler(l, span, err, "while querying Referrers")
	}
	refs = types.NewReferrersResponse()

	for _, r := range res {
		nodeType, err := r.GetStr("nodeType")
		if err != nil {
			utils.Debug(l, span, "Skipping node without nodeType: %+v", r)
			continue
		}
		switch nodeType {
		// Phase C3: content-leaf renamed from :Manifest to :Manifest.
		case "Manifest":
			digest, err := r.GetStr("digest")
			if err != nil {
				return refs, utils.ErrorHandler(l, span, err, "while getting manifest digest")
			}
			if md, found, err := neo.ManifestGetByDigest(ctx, digest); err != nil {
				return refs, utils.ErrorHandler(l, span, err, "while getting manifest data by digest")
			} else if found {
				utils.Debug(l, span, "Adding Manifest to reffers list %s with %d annotation", utils.ShortenDigest(md.Digest), len(md.Annotations.Data))
				refs.Manifests = append(refs.Manifests, md.ToOCIDescriptor())
			}
		case "ManifestList", "Index":
			digest, err := r.GetStr("digest")
			if err != nil {
				return refs, utils.ErrorHandler(l, span, err, "while getting manifestlist digest")
			}
			if mld, err := neo.ManifestListGetByDigest(ctx, digest); err != nil {
				return refs, utils.ErrorHandler(l, span, err, "while getting manifestlist data by digest")
			} else if mld != nil {
				utils.Debug(l, span, "Adding Index to reffers list %s with %d annotation", utils.ShortenDigest(mld.Digest), len(mld.Annotations))
				refs.Manifests = append(refs.Manifests, mld.ToOCIDescriptor())
			}
		default:
			utils.Debug(l, span, "Skipping unknown nodeType: %s", nodeType)
		}
	}
	utils.Debug(l, span, "Found %d reffers", len(refs.Manifests))
	return refs, nil
}
