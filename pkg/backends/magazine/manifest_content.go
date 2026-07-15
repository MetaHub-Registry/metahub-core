package magazine

import (
	"context"
	"fmt"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	ycfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameMD           = "magazine/manifest_data.go"
	FailedToGetUpstream = "failed to get upstream"
)

func (mag Magazine) cacheManifestByUpstream(ctx context.Context, up types.Upstream) (maniData types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "cacheManifestByUpstream",
	})
	defer span.End()
	utils.Info(l, span, "Caching manifest for: %s", up.GetLabel())
	uID, found, e := mag.Database.UpstreamGet(ctx, up)
	up.ID = uID
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailedToGetUpstream)
		return
	}
	utils.Debug(l, span, "Found upstream: %t / digest: %s", found, utils.ShortenDigest(up.Digest.String()))
	if found {
		if up, _, err = mag.Database.UpstreamGetByID(ctx, uID); err != nil {
			err = utils.ErrorHandler(l, span, err, "failed to get upstream by ID")
			return
		}
		// Try the MinIO cache first. If the manifest bytes exist but the
		// config blob they reference is missing (stale/incomplete cache),
		// fall through to the upstream fetch path instead of failing.
		maniData, isCached, e := mag.Minio.GetCachedManifest(ctx, up.Digest)
		if e == nil && isCached {
			maniData.Upstream = up
			return maniData, nil
		}
		if e != nil {
			utils.Warn(l, span, "cached manifest incomplete (config missing?), will re-fetch from upstream: %v", e)
		} else {
			utils.Warn(l, span, "manifest not cached in MinIO, will fetch from upstream")
		}
		// Fall through to the upstream fetch path below
	}
	utils.Debug(l, span, "Caching manifest for %-101s -> %t", up.GetLabel(), found)
	// Reaching here means the MinIO cache did not satisfy the request: either the
	// upstream node is new (!found), or it exists in the graph but its
	// manifest/config bytes were never cached (parse-collections records upstream
	// nodes without caching docker.io/gitlab.com blobs). A MinIO cache hit already
	// returned above, so in every remaining case we must (re-)fetch from the
	// upstream registry. Previously this block was gated on `if !found`, so the
	// found-but-uncached case fell through and returned a zero-value Manifest with
	// an empty Config.Digest — which downstream ManifestUpsertBackends ->
	// EnsureBlobExists("") rejects with "Object name cannot be empty", breaking
	// pull-time model injection for non-pusher profiles.
	{
		utils.Debug(l, span, "Manifest not (fully) cached, fetching from upstream")
		// ***** Manifest not already there
		b, blob, e := mag.GetManifestByUpstreamByDigest(ctx, up)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to GetManifestByUpstreamTripleByTag")
			return maniData, err
		}
		rawMani, e := minio.RawManifestFromBlob(b)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to RawManifestFromBlob")
			return maniData, err
		}
		switch blob.MediaType {
		case "application/vnd.docker.distribution.manifest.v1+prettyjws":
			err = fmt.Errorf("manifest type is not supported: %s", blob.MediaType)
			return maniData, err
		case "application/vnd.docker.distribution.manifest.v2+json",
			// OCI image manifest is structurally identical (config + layers) and is
			// what podman/buildah emit by default — treat it the same. Without this,
			// caching an OCI-format upstream (e.g. a collection's runtime image) fails
			// with "Unknown media type", so parse-collections can't bind it per profile.
			"application/vnd.oci.image.manifest.v1+json":
			up.Digest = rawMani.ConfigInfo().Digest
		default:
			return maniData, utils.NewError(l, span, "Unknown media type: %s", blob.MediaType)
		}
		// We need to make sure that the config-blob is cached
		cfgByte, cfgBlob, e := mag.GetBlobByUpstream(ctx, up)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to GetBlobByUpstreamTriple")
			return maniData, err
		}
		e = mag.Minio.PutBlob(ctx, cfgByte, cfgBlob.Digest.String())
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to PutConfig")
			return maniData, err
		}
		// The blob 'b' is the original data blob (matching the digest), it contains refrerences to the config and layers
		maniData, e = mag.Minio.NewManifestFromBytes(ctx, b)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to NewManifest_FromBytes")
			return maniData, err
		}
		if e := mag.ManifestUpsertBackends(ctx, &maniData); e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to ManifestUpsertBackends")
			return maniData, err
		}
		up.Digest = digest.Digest(maniData.Digest)
		up.Size = blob.Size
		e = mag.Minio.PutManifestByUpstream(ctx, b, up)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to PutManifest")
			return maniData, err
		}
		// Add Upstream with digest to graph
		u := types.Upstream{
			Reg:    up.Reg,
			Repo:   up.Repo,
			Tag:    up.Tag,
			Digest: up.Digest,
		}
		upID, e := mag.NeoInsertUpstream(ctx, u)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to NeoInsertUpstream")
			return maniData, err
		}
		if e := mag.Database.ManifestLinkToUpstream(ctx, maniData.ID, upID); e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to ManifestLinkToUpstream")
			return maniData, err
		}
		u.ID = upID
		maniData.Data = b
		maniData.Upstream = u
	}
	return maniData, nil
}

func (mag Magazine) ManifestUpsertBackends(ctx context.Context, md *types.Manifest) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestUpsertBackends",
	})
	defer span.End()

	if err := mag.Minio.EnsureBlobExists(ctx, md.Config.Digest, md.Config.Data); err != nil {
		return utils.ErrorHandler(l, span, err, "failed to EnsureBlobExists")
	}
	// Make sure config is in Graph. CRITICAL: adopt the existing node's
	// ID in the found branch too. Neo4j internal IDs start at 0, so a
	// zero-value md.Config.ID is indistinguishable from "the oldest node
	// in the DB". If left at 0, the downstream ManifestLinkToConfig
	// (id(c)=0) links the manifest to whatever node happens to be id 0,
	// producing a phantom config edge and a "layers from manifest don't
	// match image configuration" pull error.
	if foundCfg, found, err := mag.Database.ConfigGetByDigest(ctx, md.Config.Digest); err != nil {
		return utils.ErrorHandler(l, span, err, "failed to ConfigGetByDigest")
	} else if !found {
		cfgID, err := mag.Database.ConfigInsert(ctx, md.Config)
		if err != nil {
			return utils.ErrorHandler(l, span, err, "failed to ConfigInsert")
		}
		md.Config.ID = cfgID
	} else {
		md.Config.ID = foundCfg.ID
	}
	// Make sure layers are in. Same ID-adoption invariant as the config
	// above: set lyr.ID in the found branch so it is never left at 0.
	for idx, lyr := range md.Layers {
		if foundLyr, found, err := mag.Database.LayerGetByDigest(ctx, lyr.Digest); err != nil {
			return utils.ErrorHandler(l, span, err, "failed to LayerGetByDigest")
		} else if !found {
			lyrID, err := mag.Database.LayerPlainInsert(ctx, lyr)
			if err != nil {
				return utils.ErrorHandler(l, span, err, "failed to LayerInsert")
			}
			lyr.ID = lyrID
		} else {
			lyr.ID = foundLyr.ID
		}
		md.Layers[idx] = lyr
	}
	if e := mag.Database.ManifestGetInsert(ctx, md); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to ManifestGetInsert")
	}
	// Fetch Platform ID
	if e := mag.Database.PlatformGetInsert(ctx, &md.Platform); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to PlatformGetInsert")
	}
	return nil
}

func (mag Magazine) CachManifest(ctx context.Context, m ycfg.Manifest) (mani types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "CachManifest",
	})
	defer span.End()
	up, err := m.ToUpstream()
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to create Upstream from ycfg.Manifest")
		return
	}
	if up.Digest.String() == "" {
		utils.Debug(l, span, "digest is empty")
		blob, found, err := mag.HeadManifestByRegRepoTag(ctx, up.Reg, up.Repo, up.Tag)
		if err != nil {
			err = utils.ErrorHandler(l, span, err, "failed to HeadManifestByRegRepoTag")
			return mani, err
		} else if !found {
			err = utils.NewError(l, span, "manifest not found")
			return mani, err
		}
		up.Digest = blob.Digest
	}
	return mag.cacheManifestByUpstream(ctx, up)
}

func (mag Magazine) ManifestInsert(ctx context.Context, m *types.Manifest) (maniID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestInsert",
	})
	defer span.End()
	if m.Digest == m.Config.Digest {
		logrus.Fatalf("NeoInsertManifest: Manifest.Digest and Manifest.Config.Digest are equal - something went wrong\n Digest:    %s\n ConfigDig: %s ", m.Digest, m.Config.Digest)
	}
	if e := mag.ManifestUpsertBackends(ctx, m); e != nil {
		return 0, utils.ErrorHandler(l, span, e, "failed to ManifestUpsertBackends")
	}
	return mag.Database.ManifestInsert(ctx, m)
}

// NeoInsertManifest inserts a content-leaf Manifest node and
// links it to the wrapper :ManifestWrapper node (msID). The wrapper link is
// preserved as a compatibility shim for the upstream-pull and compute
// paths (CreateOCIv1ForProfile) that still create wrapper nodes.
func (mag Magazine) NeoInsertManifest(ctx context.Context, msID int64, m types.Manifest, props ...string) (maniID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "NeoInsertManifest",
		"maniID":  msID,
	})
	defer span.End()
	maniID, e := mag.ManifestInsert(ctx, &m)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to ManifestInsert")
		return
	}
	// Phase C3 compat: link to wrapper if provided (used by upstream-pull
	// and CreateOCIv1ForProfile which still create wrapper nodes)
	if msID > 0 {
		if e = mag.Database.LinkManifestWrapperToManifest(ctx, msID, maniID, props...); e != nil {
			utils.Debug(l, span, "LinkManifestWrapperToManifest failed (non-fatal): %v", e)
		}
	}
	return
}

// linkManifestSubject wires the OCI subject/referrer relationship for a pushed
// manifest: it inserts the subject descriptor as a Manifest node and MERGEs the
// (manifest)-[:SUBJECT_OF]->(subject) edge, so referrers(subject) resolves. It is
// shared by both push paths — StoreManifestArtifactUpload (artifact manifests)
// and StoreManifestUpload (image manifests that carry a subject, e.g. the
// model-config artefacts `metahub-admin repackage` produces) — so the edge is
// wired regardless of how the referrer manifest is classified. Returns the
// inserted subject Manifest (nil when the manifest carries no subject).
func (mag Magazine) linkManifestSubject(ctx context.Context, l *logrus.Entry, span trace.Span, mdDigest string, ociMani types.OCIManifest) (*types.Manifest, error) {
	if ociMani.Subject == nil {
		return nil, nil
	}
	utils.Debug(l, span, "Manifest has Subject: %s", utils.ShortenDigest(ociMani.Subject.Digest.String()))
	sub := types.ManifestFromOciDescriptor(*ociMani.Subject)
	subID, err := mag.Database.ManifestInsert(ctx, &sub)
	if err != nil {
		return nil, utils.ErrorHandler(l, span, err, "failed to insert subject manifest data")
	}
	utils.Debug(l, span, "Inserted Subject Manifest with ID: %d", subID)
	if err := mag.Database.LinkManifestToSubject(ctx, mdDigest, subID); err != nil {
		return nil, utils.ErrorHandler(l, span, err, "failed to link manifest data to subject")
	}
	return &sub, nil
}

// ensureBlobNodeResolvable guarantees the content-addressed :Blob node for `dig`
// exists so a manifest can link to it. The HEAD content-addressed fallback lets a
// client skip re-uploading a blob whose object survives in the bucket but whose
// :Blob graph node was wiped (e.g. the e2e --empty-graph reseed) — without this
// the manifest-link path 400s "config blob not found". Resolution order:
//  1. graph node already present -> nothing to do;
//  2. object present in the bucket -> re-materialize the node from its stat size;
//  3. OCI empty-config sentinel -> materialize the 2-byte "{}" object + node;
//  4. otherwise the blob is genuinely absent -> error (a real MANIFEST_INVALID).
func (mag Magazine) ensureBlobNodeResolvable(ctx context.Context, l *logrus.Entry, span trace.Span, dig, mediaType string) error {
	if _, found, err := mag.Database.BlobGetByDigest(ctx, dig); err != nil {
		return utils.ErrorHandler(l, span, err, "failed to BlobGetByDigest")
	} else if found {
		return nil
	}
	var size int64
	if obj, e := mag.Minio.BlobStatObj(ctx, dig); e == nil {
		size = obj.Size
		utils.Info(l, span, "blob %s missing from graph but present in bucket (%d bytes) — re-materializing :Blob node", utils.ShortenDigest(dig), size)
	} else if dig == types.EmptyConfigDigest || mediaType == types.MediaTypeOCIEmpty {
		if e2 := mag.Minio.PutBlob(ctx, types.EmptyConfigBytes, dig); e2 != nil {
			return utils.ErrorHandler(l, span, e2, "failed to materialize empty-config blob object")
		}
		size = int64(len(types.EmptyConfigBytes))
		utils.Info(l, span, "materialized OCI empty-config sentinel %s (%d bytes)", utils.ShortenDigest(dig), size)
	} else {
		return utils.NewError(l, span, "blob %s not found in graph or object store", utils.ShortenDigest(dig))
	}
	if e := mag.Database.BlobEnsureByDigest(ctx, dig, size); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to materialize :Blob node")
	}
	return nil
}

func (mag Magazine) StoreManifestArtifactUpload(ctx context.Context, req types.Request, ociMani types.OCIManifest) (md types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "StoreManifestArtifactUpload",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  req.Digest,
		"profile": req.Profile,
	})
	defer span.End()
	var found bool
	md, found, err = mag.Database.ManifestGetByDigest(ctx, req.Digest)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to ManifestGetByDigest")
		return md, err
	}
	if !found {
		utils.Debug(l, span, "Artifact Manifest not found, creating new one for digest: %s", utils.ShortenDigest(req.Digest))
	}
	// Propagate ArtifactType from the parsed OCI manifest so callers can
	// detect artifacts without re-parsing the raw bytes.
	md.ArtifactType = ociMani.ArtifactType

	utils.Debug(l, span, "Link Manifest w/ Config Blob: %s", utils.ShortenDigest(ociMani.Config.Digest.String()))
	// Content-addressed fallback: the client may have HEAD-skipped uploading the
	// config blob (its object survives a graph wipe, so BlobHandler answers the HEAD
	// 200 without a :Blob node). Re-materialize the node from the bucket — or, for
	// the OCI empty-config sentinel, synthesize it — so the link below does not
	// 400 "config blob not found".
	if err := mag.ensureBlobNodeResolvable(ctx, l, span, ociMani.Config.Digest.String(), ociMani.Config.MediaType); err != nil {
		return md, utils.ErrorHandler(l, span, err, "failed to ensure config blob resolvable")
	}
	if err := mag.Database.ManifestLinkToArtifactConfig(ctx, req.Digest, ociMani.Config); err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to ManifestLinkToArtifactConfig")
		return md, err
	}
	// Wire Slot so the artifact is reachable via the Slot-centred graph model
	mag.wireSlotOnPush(ctx, l, span, req, md.ID)
	// Handle subject/referrer relationship (shared with the image push path).
	sub, subErr := mag.linkManifestSubject(ctx, l, span, req.Digest, ociMani)
	if subErr != nil {
		return md, subErr
	}
	md.Subject = sub
	return md, nil
}

func (mag Magazine) StoreManifestUpload(ctx context.Context, req types.Request, buf []byte) (md types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "StoreManifestUpload",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  req.Digest,
		"profile": req.Profile,
	})
	defer span.End()
	dig := digest.FromBytes(buf)
	req.Digest = dig.String()

	var ociMani types.OCIManifest

	// Put Manifest Blob into Minio and insert content-leaf
	md, ociMani, e := mag.storeManifestUploadOCIManifest(ctx, req, buf)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to mag.storeManifestUploadOCIManifest")
		return
	}
	utils.Debug(l, span, "ociMani.Config.MediaType:%s", ociMani.Config.MediaType)
	label := fmt.Sprintf("%s/%s:%s", req.Host, req.Repo, req.Tag)
	utils.Debug(l, span, "init Manifest with label '%s' and digest '%s'", label, utils.ShortenDigest(req.Digest))
	md.Digest = req.Digest
	md.Label = label
	// Do we have a typical OCI Manifest to run a container or an artifact manifest?
	if ociMani.IsArtifact() {
		utils.Info(l, span, "Storing Artifact Manifest for Digest: %s", utils.ShortenDigest(md.Digest))
		return mag.StoreManifestArtifactUpload(ctx, req, ociMani)
	}

	// Fetch Config
	cfg, v1Desc, v1Img, e := mag.ConfigGetByManifest(ctx, req, ociMani)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to mag.ConfigGetByManifest")
		return
	}
	cfg.MediaType = ociMani.Config.MediaType
	if v1Desc == nil && v1Img == nil {
		err = utils.NewError(l, span, "failed to find config for manifest")
		return
	}
	utils.Debug(l, span, "Found Config: ID:%d Digest:%s", cfg.ID, cfg.Digest)
	md.Config = cfg
	// Content-addressed fallback (parity with the ORAS artifact path): guarantee the
	// config :Blob node exists even if the client HEAD-skipped its upload after a
	// graph wipe, so ManifestLinkToConfig (MATCH (c:Blob {digest})) resolves instead
	// of silently linking nothing.
	if e = mag.ensureBlobNodeResolvable(ctx, l, span, cfg.Digest, cfg.MediaType); e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to ensure config blob resolvable")
		return
	}
	// Link Manifest to Config
	utils.Debug(l, span, "Linking Manifest to Config: '%d' -> '%d'", md.ID, cfg.ID)
	e = mag.Database.ManifestLinkToConfig(ctx, md.ID, cfg)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to mag.LinkManifestToConfig")
		return
	}
	/// Extract Platform (Will be put in by ConfigGetInsertByOCIManifest in case it is embedded)
	if v1Img.Architecture == "" {
		l.Infof("cfg: %v", cfg)
		l.Fatal("architecture should not be empty")
	}
	plat := types.Platform{
		Arch:    v1Img.Architecture,
		OS:      v1Img.OS,
		Variant: v1Img.Variant,
	}
	err = mag.Database.PlatformGetInsert(ctx, &plat)
	if err != nil {
		err = errors.Wrap(err, "failed to Database.PlatformGetInsert")
		return
	}
	err = mag.Database.LinkManifestToPlatform(ctx, md.ID, plat.ID)
	if err != nil {
		err = errors.Wrap(err, "failed to Database.LinkManifestToPlatform")
		return
	}
	md.Platform = plat
	/*err = mag.Database.FilterUpdatePlatform(ctx, plat)
	if err != nil {
		err = errors.Wrap(err, "failed to SQlite.UpdatePlatformFilter")
		return
	}
	*/
	req.Platform = plat.GetTag()
	// Fetch/Create Upstream
	up, _, err := mag.Database.UpstreamGetInsertByRequest(ctx, req)
	if err != nil {
		err = errors.Wrap(err, "failed to Upstream.GetCreateByRequest")
		return
	}
	md.Upstream = up
	// Path Z Phase C3: the legacy wrapper path (ManifestWrapperLinkToUpstream +
	// CollectionGetCreateByRepoTag + LinkCollection{,Tag}ToManifest) is
	// deleted. The content-leaf :Manifest node is already linked to
	// :Upstream via [:ORIGINAL] inside createManifest. The Slot-based
	// attribution is the sole remaining persistence path.
	mag.wireSlotOnPush(ctx, l, span, req, md.ID)
	// Wire the OCI subject/referrer edge for image manifests that carry one
	// (e.g. the model-config artefacts `metahub-admin repackage` produces, whose
	// subject is the model-layer manifest), so referrers(subject) — and model
	// compositions — resolve. Previously only StoreManifestArtifactUpload did
	// this, so subject-bearing *image* manifests silently lost the edge.
	if sub, subErr := mag.linkManifestSubject(ctx, l, span, md.Digest, ociMani); subErr != nil {
		return md, subErr
	} else if sub != nil {
		md.Subject = sub
	}
	return
}

// wireSlotOnPush creates a :Slot node for the (repo, tag, profile) triple
// and appends a :HAS_PUSHED edge to the content-leaf Manifest. It is the
// Phase A wiring added by Path Z task 6 and runs IN ADDITION TO the
// legacy wrapper path (CollectionGetCreateByRepoTag +
// LinkCollection{,Tag}ToManifest) during the migration window. Phase C
// of task 6 deletes the legacy path and leaves only the Slot wiring.
//
// The function is intentionally non-fatal: any failure is logged at
// debug level but the caller's push still succeeds because the legacy
// wrapper path has already persisted the manifest and the pull handler
// can still walk via the wrapper. This defensiveness is belt-and-braces
// against neo4j transient issues during the migration window; once the
// legacy path is deleted in Phase C the non-fatal semantics will be
// upgraded to hard errors.
//
// Gated on req.Profile != "": the upstream storeManifestUploadOCIManifest
// helper already rejects empty profiles with "profile ” not found", but
// the guard stays as defence-in-depth against future callers that might
// bypass the profile lookup.
func (mag Magazine) wireSlotOnPush(ctx context.Context, l *logrus.Entry, span trace.Span, req types.Request, manifestDataID int64) {
	if req.Profile == "" {
		return
	}
	slot, slotErr := mag.Database.SlotGetCreate(ctx, req.Repo, req.Tag, req.Profile)
	if slotErr != nil {
		utils.Debug(l, span, "SlotGetCreate failed (non-fatal, legacy path still persisted): %v", slotErr)
		return
	}
	// Ensure the Repo -[:HAS_TAG]-> Tag edge exists so OCI manifest GET
	// (ManifestExistsContent) can resolve tags after a push.
	if tagErr := mag.Database.RepoLinkTag(ctx, req.Repo, req.Tag); tagErr != nil {
		utils.Debug(l, span, "RepoLinkTag failed (non-fatal): %v", tagErr)
	}
	if pushErr := mag.Database.SlotAppendPushedManifest(ctx, slot.ID, manifestDataID); pushErr != nil {
		utils.Debug(l, span, "SlotAppendPushedManifest failed (non-fatal, Slot node still created): %v", pushErr)
		return
	}
	utils.Debug(l, span, "Slot wired: slotID=%d (repo=%s tag=%s profile=%s) -> HAS_PUSHED manifest %d",
		slot.ID, req.Repo, req.Tag, req.Profile, manifestDataID)
}

func (mag Magazine) ManifestGetByID(ctx context.Context, mdID int64) (md types.Manifest, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestGetByID",
		"maniDataID": mdID,
	})
	defer span.End()
	md, mdFound, e := mag.Database.ManifestGetByID(ctx, mdID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to ManifestGetByID")
		return
	}
	if !mdFound {
		err = utils.NewError(l, span, "manifest data not found")
		return
	}
	// Get COnfig Blob
	utils.Debug(l, span, "getting config from storage for digest '%s'", utils.ShortenDigest(md.Config.Digest))
	cfg, e := mag.Minio.ConfigGetObj(ctx, md.Config.Digest)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get config from storage")
		return
	}
	utils.Debug(l, span, "Env for config: %v", cfg.Cfg.Config.Env)
	md.Config = cfg
	found = true
	return
}
