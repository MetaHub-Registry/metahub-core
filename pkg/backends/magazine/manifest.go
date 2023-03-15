package magazine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	ycfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (mag Magazine) GetManifest(ctx context.Context, m ycfg.Manifest) (mani types.ManifestData, err error) {
	up, err := m.ToUpstreamTriple()
	if err != nil {
		logrus.Error(err)
		return
	}
	// The Manifest cached has a Data filed with the original data blob (matching the digest)
	// in addition it has fields populated with the parsed data.
	mani, isCached, err := mag.Minio.GetManifest(ctx, up)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !isCached {
		err = errors.New("manifest is not cached")
		return
	}
	mani.Upstream = types.Upstream{
		Reg:    up.Reg,
		Repo:   up.Repo,
		Tag:    up.Tag,
		Digest: up.Digest,
	}
	//mani.Annotations = types.NewAnnotations()
	/// Config
	cfg, found, err := mag.Minio.GetTypesConfig(ctx, mani.Config.Digest)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !found {
		err = errors.New("config is not cached")
		return
	}
	mani.Config = cfg
	/*anno, err := mag.Manifest_LabelsToAnnotations(mani)
	if err != nil {
		logrus.Error(err)
		return
	}
	err = mani.UpdateAnnotations(anno)
	if err != nil {
		logrus.Error(err)
		return
	}
	*/
	return
}

/*
func (mag Magazine) Manifest_LabelsToAnnotations(mani types.Manifest) (anno types.Annotations, err error) {
	return mani.Config.LabelsToAnnotations()
}
*/

func (mag Magazine) CachManifest(ctx context.Context, m ycfg.Manifest) (mani types.ManifestData, err error) {
	up, err := m.ToUpstreamTriple()
	if err != nil {
		logrus.Error(err)
		return
	}
	// The Manifest cached has a Data filed with the original data blob (matching the digest)
	// in addition it has fields populated with the parsed data.
	mani, isCached, err := mag.Minio.GetManifest(ctx, up)
	if err != nil {
		logrus.Error(err)
		return
	}
	logrus.Infof("Caching manifest for %-101s -> %t", up.GetLabel(), isCached)
	if !isCached {
		/****** Manifest not already there
		TODO: Depending on the media type, we are going to create different nodes within the graph
		*/
		b, blob, err := upstream.GetManifestByUpstreamTripleByTag(ctx, up)
		if err != nil {
			err = errors.Wrapf(err, "failed to GetManifestByUpstreamTripleByTag")
			logrus.Error(err)
			return mani, err
		}
		rawMani, err := minio.RawManifestFromBlob(b)
		if err != nil {
			logrus.Error(err)
			return mani, err
		}
		switch blob.MediaType {
		case "application/vnd.docker.distribution.manifest.v1+prettyjws":
			err = fmt.Errorf("manifest type is not supported: %s", blob.MediaType)
			return mani, err
		case "application/vnd.docker.distribution.manifest.v2+json":
			up.Digest = rawMani.ConfigInfo().Digest
		default:
			logrus.Fatalf("Unknown media type: %s", blob.MediaType)
		}
		//logrus.Fatalf("blobMeta: %v", blob)
		// We need to make sure that the config-blob is cached
		cfgByte, cfgBlob, err := upstream.GetBlobByUpstreamTriple(ctx, up)
		if err != nil {
			err = errors.Wrapf(err, "failed to GetBlobByUpstreamTriple")
			logrus.Error(err)
			return mani, err
		}
		err = mag.Minio.PutConfig(ctx, cfgByte, cfgBlob.Digest.String())
		if err != nil {
			logrus.Error(err)
			return mani, err
		}
		// The blob 'b' is the original data blob (matching the digest), it contains refrerences to the config and layers
		mani, err = mag.Minio.NewManifestFromBytes(b)
		if err != nil {
			logrus.Error(err)
			return mani, err
		}
		up.Digest = digest.FromBytes(b)
		up.Size = blob.Size
		err = mag.Minio.PutManifest(ctx, b, up)
		if err != nil {
			logrus.Error(err)
			return mani, err
		}
		mani.Data = b
		if err != nil {
			logrus.Error(err)
			return mani, err
		}
	}
	mani.Upstream = types.Upstream{
		Reg:  up.Reg,
		Repo: up.Repo,
		Tag:  up.Tag,
	}
	return
}

func (mag Magazine) ManifestData_Insert(ctx context.Context, m types.ManifestData) (maniID int64, err error) {
	if m.Digest == m.Config.Digest {
		logrus.Fatalf("NeoInsertManifest: Manifest.Digest and Manifest.Config.Digest are equal - something went wrong\n Digest:    %s\n ConfigDig: %s ", m.Digest, m.Config.Digest)
	}

	return mag.Database.ManifestData_Insert(ctx, m)
}

// TODO: Rename to NeoInsertManifestData
func (mag Magazine) NeoInsertManifestData(ctx context.Context, msID int64, m types.ManifestData, props ...string) (maniID int64, err error) {
	maniID, err = mag.ManifestData_Insert(ctx, m)
	if err != nil {
		logrus.Error(err)
		return
	}
	// Link Manifest(Standin) to ManifestData
	err = mag.LinkManifestToManifestData(ctx, msID, maniID, props...)
	return
}

func (mag Magazine) NeoLinkManifestToUpstream(ctx context.Context, maniID, upID int64) (err error) {
	return mag.Database.Manifest_LinkToUpstream(ctx, maniID, upID)
}

func (mag Magazine) LinkManifestToManifestData(ctx context.Context, maniID, maniDataID int64, props ...string) (err error) {
	return mag.Database.LinkManifestToManifestData(ctx, maniID, maniDataID, props...)
}

func (mag Magazine) NeoLinkManifestDataToConfig(ctx context.Context, maniID, cfgID int64) (err error) {
	return mag.Database.ManifestData_LinkToConfig(ctx, maniID, cfgID)
}

func (mag Magazine) NeoLinkManifestToLayer(ctx context.Context, maniID, lID int64, idx int) (err error) {
	return mag.Database.ManifestData_LinkToLayer(ctx, maniID, lID, idx)
}

func (mag Magazine) NeoLinkManifestToPlatform(ctx context.Context, maniID, platID int64) (err error) {
	return mag.Database.LinkManifestToPlatform(ctx, maniID, platID)
}

func (mag Magazine) Manifest_GetByHTTPRequest(ctx context.Context, r *http.Request) (m types.ManifestData, found bool, err error) {
	req := types.NewRequest(ctx, r)
	if req.Profile == "" {
		return
	}
	ctx, span := tracer.Start(ctx, "Manifest_GetByHTTPRequest")
	span.SetAttributes(
		attribute.Key("method").String(r.Method),
		attribute.Key("repo").String(req.Repo),
	)
	defer span.End()
	msg := fmt.Sprintf("%-5s ManifestHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	if req.Profile != "generic" {
		msg += fmt.Sprintf(" - profile: %s", req.Profile)
	}
	logrus.Info(msg)
	m, found, err = mag.Manifest_GetByRequest(ctx, req)
	if err != nil {
		return
	}
	return
}

func (mag Magazine) Manifest_DeleteByHTTPRequest(ctx context.Context, r *http.Request) (err error) {
	req := types.NewRequest(ctx, r)
	msg := fmt.Sprintf("%-5s DeleteManifestHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	logrus.Warn(msg)
	found, err := mag.RemoveManifestByRequest(req)
	if err != nil {
		return
	}
	if !found || req.Tag != "" {
		//Not a plain Manifest, let's try to delete a Collection
		logrus.Warn("Not a plain Manifest, let's try to delete a Collection")
		found, err = mag.RemoveCollectionByRequest(req)
		if err != nil {
			return
		}
		if !found {
			err = errors.New("Collection not found")
		}
		return
	}
	return
}

func (mag Magazine) Manifest_GetByRequest(ctx context.Context, req types.Request) (m types.ManifestData, found bool, err error) {
	ctx, span := tracer.Start(ctx, "Manifest_GetByRequest")
	defer span.End()
	// Fetch Manifest from SQLite
	m, found, err = mag.Database.ManifestData_GetTypes_ByRequest(ctx, req)
	if err != nil {
		err = errors.Wrap(err, "failed to get Manifest from Database")
		return
	}
	if !found {
		return
	}
	m, err = mag.Minio.ManifestGetObj(ctx, m.Digest)
	return
}

// storeManifestUpload_OCIManifest
func (mag Magazine) storeManifestUpload_OCIManifest(ctx context.Context, req types.Request, buf []byte) (mani types.Manifest, ociMani types.OCIManifest, err error) {
	ctx, span := tracer.Start(ctx, "storeManifestUpload_OCIManifest")
	defer span.End()
	ociMani = types.OCIManifest{
		MediaType: req.ContentType,
	}
	err = json.Unmarshal(buf, &ociMani)
	if err != nil {
		err = errors.Wrapf(err, "failed to unmarshal OCI Manifest '%s'", string(buf))
		return mani, ociMani, err
	}
	dig := digest.FromBytes(buf)
	req.Digest = dig.String()
	if req.Digest != "" && req.Digest != dig.String() {
		err = fmt.Errorf("digest mismatch: %s != %s", req.Digest, dig.String())
		return
	}
	err = mag.Minio.ManifestsPutObj(ctx, dig.String(), buf)
	if err != nil {
		logrus.Error(err)
		return
	}
	// Store Manifest (the standin under which all ManifestData are stored)
	up := types.Upstream{
		Reg:  req.Host,
		Repo: req.Repo,
		Tag:  req.Tag,
	}
	mani = types.Manifest{
		Upstream:  up,
		Manifests: []types.ManifestData{},
	}

	mID, err := mag.Database.Manifest_GetInsert(ctx, mani)
	if err != nil {
		err = errors.Wrap(err, "failed to get Manifest from Database")
		return
	}
	mani.ID = mID
	lyrs := []types.Layer{}
	for _, l := range ociMani.Layers {
		lyrs = append(lyrs, types.Layer{
			Digest:    l.Digest.String(),
			MediaType: l.MediaType,
			Size:      l.Size,
		})
	}
	prf, found, e := mag.Database.Profile_Get_ByLabel(ctx, req.Profile)
	if e != nil {
		err = errors.Wrapf(e, "failed to get profile '%s' from database", prf.Label)
		return
	}
	if !found {
		err = fmt.Errorf("profile '%s' not found", prf.Label)
		return
	}
	md, found, e := mag.Database.ManifestData_GetPushed_ByProfile(ctx, up, prf)
	if e != nil {
		err = errors.Wrapf(e, "failed to get ManifestData for profile '%s' from database", prf.Label)
		return
	}
	if found {
		msg := fmt.Sprintf("manifest '%s' already pushed for profile: '%s'", up.String(), prf.Label)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		if dig.String() != md.Digest {
			msg = fmt.Sprintf("incoming manifest ('%s') is different from already present manifest ('%s')", utils.ShortenDigest(dig.String()), utils.ShortenDigest(md.Digest))
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", msg),
			))
			// TODO: Remove manifestdata already pushed
			err = mag.Database.ManifestData_Delete(ctx, md)

		} else {
			msg = fmt.Sprintf("incoming manifest ('%s') is the same as already present manifest ('%s')", utils.ShortenDigest(dig.String()), utils.ShortenDigest(md.Digest))
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", msg),
			))
		}

	}

	// Store ManifestData (the actual Manifest with digest and all)
	md = types.ManifestData{
		Digest:    dig.String(),
		MediaType: ociMani.MediaType,
		Data:      buf,
		Size:      int64(len(buf)),
		Layers:    lyrs,
	}
	err = mag.Database.ManifestData_GetInsert(ctx, &md)
	if err != nil {
		err = errors.Wrap(err, "failed to get ManifestData from Database")
		return
	}
	mani.Manifests = append(mani.Manifests, md)
	err = mag.LinkManifestToManifestData(ctx, mID, md.ID, "ORIGINAL")
	if err != nil {
		err = errors.Wrap(err, "failed to mag.LinkManifestToManifestData")
		return
	}
	err = mag.Database.LinkManifestDataToProfile(ctx, md.ID, prf.ID)
	if err != nil {
		err = errors.Wrap(err, "failed to mag.LinkManifestToManifestData")
		return
	}
	// Remove orhpaned layers

	err = mag.Layers_DeleteUnused(ctx)
	if err != nil {
		err = errors.Wrap(err, "failed to delete unused layers")
		span.RecordError(err)
		return
	}
	// Remove orhpaned config
	err = mag.Configs_DeleteUnused(ctx)
	if err != nil {
		err = errors.Wrap(err, "failed to delete unused config")
		span.RecordError(err)
		return
	}
	return
}

func (mag Magazine) StoreManifestUpload(ctx context.Context, req types.Request, buf []byte) (md types.ManifestData, err error) {
	/// TODO: Can this be dratically simplified?
	ctx, span := tracer.Start(ctx, "StoreManifestUpload")
	defer span.End()
	span.SetAttributes(
		attribute.Key("repo").String(req.Repo),
		attribute.Key("body-length").Int(len(buf)),
		attribute.Key("profile").String(req.Profile),
	)
	var ociMani types.OCIManifest
	// Put Manifest Blob into Minio
	m, ociMani, err := mag.storeManifestUpload_OCIManifest(ctx, req, buf)
	switch len(m.Manifests) {
	default:
		err = fmt.Errorf("expect to have to store Manifest")
		return
	case 1:
		md = m.Manifests[0]
	}
	if err != nil {
		err = errors.Wrap(err, "failed to mag.storeManifestUpload_OCIManifest")
		return
	}
	// Fetch Config
	cfg, v1Img, found, err := mag.Config_Get_ByManifest(ctx, req, ociMani)
	if err != nil {
		err = errors.Wrap(err, "failed to mag.Config_Get_ByManifest")
		return
	}
	if !found {
		err = fmt.Errorf("failed to find config for manifest")
		return
	}
	//logrus.Infof("Found Config: ID:%d #Labels:%d", cfg.ID, len(cfg.Labels.Map))
	md.Config = cfg
	// Link Manifest to Config
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", fmt.Sprintf("Linking Manifest to Config: '%d' -> '%d'", md.ID, cfg.ID)),
	))
	err = mag.Database.ManifestData_LinkToConfig(ctx, md.ID, cfg.ID)
	if err != nil {
		err = errors.Wrap(err, "failed to mag.LinkManifestToConfig")
		return
	}
	/// Extract Platform (Will be put in by Config_GetInsert_ByOCIManifest in case it is embedded)
	if v1Img.Architecture == "" {
		logrus.Infof("cfg: %v", cfg)
		logrus.Fatal("architecture should not be empty")
	}
	plat := types.Platform{
		Arch:    v1Img.Architecture,
		OS:      v1Img.OS,
		Variant: v1Img.Variant,
	}
	err = mag.Database.Platform_GetInsert(&plat)
	if err != nil {
		err = errors.Wrap(err, "failed to Database.Platform_GetInsert")
		return
	}
	err = mag.Database.LinkManifestToPlatform(ctx, m.ID, plat.ID)
	if err != nil {
		err = errors.Wrap(err, "failed to Database.LinkManifestToPlatform")
		return
	}
	md.Platform = plat
	/*err = mag.Database.Filter_UpdatePlatform(ctx, plat)
	if err != nil {
		err = errors.Wrap(err, "failed to SQlite.UpdatePlatformFilter")
		return
	}
	*/
	req.Platform = plat.GetTag()
	// Fetch/Create Upstream
	up, _, err := mag.Database.Upstream_GetInsert_ByRequest(req)
	if err != nil {
		err = errors.Wrap(err, "failed to Upstream.GetCreateByRequest")
		return
	}
	m.Upstream = up
	err = mag.Database.Manifest_LinkToUpstream(ctx, m.ID, up.ID)
	if err != nil {
		err = errors.Wrap(err, "failed to LinkManifestToUpstream")
		return
	}
	col, colFound, err := mag.Database.Collection_GetCreate_ByRepoTag(ctx, req.Repo, req.Tag)
	if err != nil {
		err = errors.Wrap(err, "failed to Collection.GetCreateByRepoTag")
		return
	}
	_ = colFound
	err = mag.Database.LinkCollectionToManifest(ctx, col.ID, m.ID)
	if err != nil {
		err = errors.Wrap(err, "failed to LinkCollectionToManifest")
		return
	}
	err = mag.Database.LinkCollectionTagToManifest(ctx, col.ColTag.ID, m.ID)
	if err != nil {
		err = errors.Wrap(err, "failed to LinkCollectionTagToManifest")
		return
	}
	// TODO: Compute Manifests
	//err = mag.ComputeManifest(ctx, m)
	// TODO: Compute ManifestLists
	/*
		err = mag.Database.ComputeManifestlists_ForCollection(ctx, col)
		if err != nil {
			err = errors.Wrap(err, "failed to ComputeManifestlists_ForCollection")
			return
		}
	*/
	return
}

func (mag Magazine) RemoveManifestByRequest(req types.Request) (found bool, err error) {
	ctx := context.Background()
	m := types.ManifestData{
		Upstream: types.Upstream{
			Repo: req.Repo,
			Tag:  req.Tag,
		},
		Digest: req.Digest,
	}
	found, err = mag.Database.ManifestData_Remove(ctx, req)
	if err != nil {
		err = errors.Wrap(err, "failed to Database.Manifest_Remove")
		return
	}
	if found {
		// FOUND: Remove Manifest Blob
		e := mag.Minio.ManifestRemoveObj(ctx, m.Digest)
		if e != nil {
			e = errors.Wrapf(e, "while in RemoveManifestByRequest (we won't fail here tho)")
			logrus.Error(e)
			return
		}
	}
	return
}

// ComputeManifests looks for changes to a manifest (e.g. new annotations), creates new config blobs and subsequently updates the manifest.
func (mag Magazine) ComputeManifest(ctx context.Context, mani types.Manifest) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/manifest.go",
		"func":    "ComputeManifest",
		"mani":    mani.ID,
		"name":    mani.Upstream.String(),
	})
	ctx, span := tracer.Start(ctx, "ComputeManifest")
	defer span.End()
	span.SetAttributes(
		attribute.Key("mani.id").Int64(mani.ID),
		attribute.Key("mani.name").String(mani.Upstream.String()),
	)
	l.Infof("Manifest: %s/%s:%s", mani.Upstream.Reg, mani.Upstream.Repo, mani.Upstream.Tag)
	for _, maniData := range mani.Manifests {
		if maniData.MediaType == v1.MediaTypeImageManifest {
			msg := fmt.Sprintf("[SKIP] ManifestData %s (%-10s): %s (Annotations: %d, Layers:%d)", utils.ShortenDigest(maniData.Digest), maniData.Origin, maniData.MediaType, len(mani.Annotations.Keys), len(maniData.Layers))
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", msg),
			))
			l.Info(msg)
			continue
		}
		msg := fmt.Sprintf("ManifestData %s (%-10s): %s (Annotations: %d, Layers:%d)", utils.ShortenDigest(maniData.Digest), maniData.Origin, maniData.MediaType, len(mani.Annotations.Keys), len(maniData.Layers))
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Info(msg)
		if len(mani.Annotations.Keys) > 0 {
			l.Infof("  >> Annotations: %v", mani.Annotations.Keys)
		}
		///TODO: Need to add the annotations to the config
		err = mag.CreateOCIv1(ctx, mani, maniData)
		if err != nil {
			l.Error(err)
			return
		}
	}
	return
}

// ComputeManifests looks for changes to a manifest (e.g. new annotations), creates new config blobs and subsequently updates the manifest.
func (mag Magazine) ComputeManifests(ctx context.Context) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/manifest.go",
		"func":    "ComputeManifests",
	})
	ctx, span := tracer.Start(ctx, "ComputeManifests")
	defer span.End()
	// 	1. Get all manifest(standins)
	manis, err := mag.Database.Manifest_GetAll(ctx)
	if err != nil {
		l.Error(err)
		return
	}
	for _, mani := range manis {
		err = mag.ComputeManifest(ctx, mani)
		if err != nil {
			l.Error(err)
			return
		}
	}
	return
}
