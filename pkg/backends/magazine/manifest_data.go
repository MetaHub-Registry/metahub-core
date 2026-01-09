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
)

const (
	pkgNameMD           = "magazine/manifest_data.go"
	FailedToGetUpstream = "failed to get upstream"
)

func (mag Magazine) cacheManifestByUpstream(ctx context.Context, up types.Upstream) (maniData types.ManifestData, err error) {
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
		// TODO: 2021-04-05 we need an option to invalidate the cached upstream objects
		maniData, isCached, e := mag.Minio.GetCachedManifest(ctx, up.Digest)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get cached manifest")
			return maniData, err
		}
		if !isCached {
			err = utils.NewError(l, span, "manifest is not cached")
		}
		maniData.Upstream = up
		return maniData, err
	}
	utils.Debug(l, span, "Caching manifest for %-101s -> %t", up.GetLabel(), found)
	if !found {
		utils.Debug(l, span, "Manifest not cached, yet")
		// ***** Manifest not already there
		b, blob, e := mag.GetManifest_ByUpstreamByDigest(ctx, up)
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
		case "application/vnd.docker.distribution.manifest.v2+json":
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
		maniData, e = mag.Minio.NewManifestDataFromBytes(ctx, b)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to NewManifestData_FromBytes")
			return maniData, err
		}
		if e := mag.ManifestDataUpsertBackends(ctx, &maniData); e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to ManifestDataUpsertBackends")
			return maniData, err
		}
		up.Digest = digest.Digest(maniData.Digest)
		up.Size = blob.Size
		e = mag.Minio.PutManifest_ByUpstream(ctx, b, up)
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
		if e := mag.Database.ManifestDataLinkToUpstream(ctx, maniData.ID, upID); e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to ManifestDataLinkToUpstream")
			return maniData, err
		}
		u.ID = upID
		maniData.Data = b
		maniData.Upstream = u
	}
	return maniData, nil
}

func (mag Magazine) ManifestDataUpsertBackends(ctx context.Context, md *types.ManifestData) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataUpsertBackends",
	})
	defer span.End()

	if err := mag.Minio.EnsureBlobExists(ctx, md.Config.Digest, md.Config.Data); err != nil {
		return utils.ErrorHandler(l, span, err, "failed to EnsureBlobExists")
	}
	// Make sure config is in Graph
	if _, found, err := mag.Database.ConfigGetByDigest(ctx, md.Config.Digest); err != nil {
		return utils.ErrorHandler(l, span, err, "failed to ConfigGetByDigest")
	} else if !found {
		cfgID, err := mag.Database.ConfigInsert(ctx, md.Config)
		if err != nil {
			return utils.ErrorHandler(l, span, err, "failed to ConfigInsert")
		}
		md.Config.ID = cfgID
	}
	// Make sure layers are in
	for idx, lyr := range md.Layers {
		if _, found, err := mag.Database.LayerGetByDigest(ctx, lyr.Digest); err != nil {
			return utils.ErrorHandler(l, span, err, "failed to LayerGetByDigest")
		} else if !found {
			lyrID, err := mag.Database.Layer_PlainInsert(ctx, lyr)
			if err != nil {
				return utils.ErrorHandler(l, span, err, "failed to LayerInsert")
			}
			lyr.ID = lyrID
		}
		md.Layers[idx] = lyr
	}
	if e := mag.Database.ManifestData_GetInsert(ctx, md); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to ManifestData_GetInsert")
	}
	// Fetch Platform ID
	if e := mag.Database.Platform_GetInsert(ctx, &md.Platform); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to Platform_GetInsert")
	}
	return nil
}

func (mag Magazine) CachManifest(ctx context.Context, m ycfg.Manifest) (mani types.ManifestData, err error) {
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

func (mag Magazine) ManifestData_Insert(ctx context.Context, m types.ManifestData) (maniID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestData_Insert",
	})
	defer span.End()
	if m.Digest == m.Config.Digest {
		logrus.Fatalf("NeoInsertManifest: Manifest.Digest and Manifest.Config.Digest are equal - something went wrong\n Digest:    %s\n ConfigDig: %s ", m.Digest, m.Config.Digest)
	}
	if e := mag.ManifestDataUpsertBackends(ctx, &m); e != nil {
		return 0, utils.ErrorHandler(l, span, e, "failed to ManifestDataUpsertBackends")
	}
	return mag.Database.ManifestData_Insert(ctx, m)
}

func (mag Magazine) NeoInsertManifestData(ctx context.Context, msID int64, m types.ManifestData, props ...string) (maniID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "NeoInsertManifestData",
		"maniID":  msID,
	})
	defer span.End()
	maniID, e := mag.ManifestData_Insert(ctx, m)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to ManifestData_Insert")
		return
	}
	// Link Manifest(Standin) to ManifestData
	e = mag.LinkManifestToManifestData(ctx, msID, maniID, props...)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to LinkManifestToManifestData")
		return
	}
	return
}

func (mag Magazine) StoreManifestUpload(ctx context.Context, req types.Request, buf []byte) (md types.ManifestData, err error) {
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

	// Put Manifest Blob into Minio
	m, ociMani, e := mag.storeManifestUpload_OCIManifest(ctx, req, buf)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to mag.storeManifestUpload_OCIManifest")
		return
	}
	utils.Debug(l, span, "ociMani.Config.MediaType:%s", ociMani.Config.MediaType)
	switch len(m.Manifests) {
	default:
		err = utils.NewError(l, span, "expect to have to store Manifest")
		return
	case 1:
		label := fmt.Sprintf("%s/%s:%s", req.Host, req.Repo, req.Tag)
		utils.Debug(l, span, "init ManifestData with label '%s' and digest '%s' from first manifest in storeManifestUpload_OCIManifest", label, utils.ShortenDigest(req.Digest))
		md = m.Manifests[0]
		md.Digest = req.Digest
		md.Label = label
	}
	// Fetch Config
	cfg, v1Img, found, e := mag.ConfigGetByManifest(ctx, req, ociMani)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to mag.ConfigGetByManifest")
		return
	}

	cfg.MediaType = ociMani.Config.MediaType
	if !found {
		err = utils.NewError(l, span, "failed to find config for manifest")
		return
	}
	utils.Debug(l, span, "Found Config: ID:%d Digest:%s", cfg.ID, cfg.Digest)
	md.Config = cfg
	// Link Manifest to Config
	utils.Debug(l, span, "Linking Manifest to Config: '%d' -> '%d'", md.ID, cfg.ID)
	e = mag.Database.ManifestDataLinkToConfig(ctx, md.ID, cfg)
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
	err = mag.Database.Platform_GetInsert(ctx, &plat)
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
	up, _, err := mag.Database.Upstream_GetInsert_ByRequest(ctx, req)
	if err != nil {
		err = errors.Wrap(err, "failed to Upstream.GetCreateByRequest")
		return
	}
	m.Upstream = up
	md.Upstream = up
	err = mag.Database.ManifestLinkToUpstream(ctx, m.ID, up.ID)
	if err != nil {
		err = errors.Wrap(err, "failed to LinkManifestToUpstream")
		return
	}
	col, colFound, err := mag.Database.CollectionGetCreateByRepoTag(ctx, req.Repo, req.Tag)
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
	return
}

func (mag Magazine) ManifestDataGetByID(ctx context.Context, mdID int64) (md types.ManifestData, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestDataGetByID",
		"maniDataID": mdID,
	})
	defer span.End()
	md, mdFound, e := mag.Database.ManifestDataGetByID(ctx, mdID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to ManifestDataGetByID")
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
