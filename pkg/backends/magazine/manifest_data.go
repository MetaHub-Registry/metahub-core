package magazine

import (
	"context"
	"fmt"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	ycfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

const (
	pkgNameMD = "magazine/manifest_data.go"
)

func (mag Magazine) CachManifest(ctx context.Context, m ycfg.Manifest) (mani types.ManifestData, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/manifest.go",
		"func":    "CachManifest",
	})
	ctx, span := tracer.Start(ctx, "CachManifest")
	defer span.End()
	up, err := m.ToUpstreamTriple()
	if err != nil {
		l.Error(err)
		return
	}
	req := types.Request{
		Host: up.Reg,
		Repo: up.Repo,
		Tag:  up.Tag,
	}
	u, found, e := mag.Database.Upstream_Get_ByRequest(ctx, req)
	if e != nil {
		err = errors.Wrap(e, "failed to get upstream")
		l.Error(err)
		return
	}
	if found {
		// TODO: 2021-04-05 we need an option to invalidate the cached upstream objects
		mani, isCached, e := mag.Minio.GetCachedManifest(ctx, u.Digest)
		if e != nil {
			err = errors.Wrap(e, "failed to get upstream")
			l.Error(err)
			return mani, err
		}
		if !isCached {
			err = errors.New("manifest is not cached")
		}
		return mani, err
	}
	if err != nil {
		l.Error(err)
		return
	}
	utils.Debug(l, span, "Caching manifest for %-101s -> %t", up.GetLabel(), found)
	if !found {
		l.Warnf("Manifest not cached, yet")
		// ***** Manifest not already there
		b, blob, e := upstream.GetManifestByUpstreamTripleByTag(ctx, up)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to GetManifestByUpstreamTripleByTag")
			return mani, err
		}
		rawMani, e := minio.RawManifestFromBlob(b)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to RawManifestFromBlob")
			return mani, err
		}
		switch blob.MediaType {
		case "application/vnd.docker.distribution.manifest.v1+prettyjws":
			err = fmt.Errorf("manifest type is not supported: %s", blob.MediaType)
			return mani, err
		case "application/vnd.docker.distribution.manifest.v2+json":
			up.Digest = rawMani.ConfigInfo().Digest
		default:
			l.Fatalf("Unknown media type: %s", blob.MediaType)
		}
		// We need to make sure that the config-blob is cached
		cfgByte, cfgBlob, e := upstream.GetBlobByUpstreamTriple(ctx, up)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to GetBlobByUpstreamTriple")
			return mani, err
		}
		e = mag.Minio.PutConfig(ctx, cfgByte, cfgBlob.Digest.String())
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to PutConfig")
			return mani, err
		}
		// The blob 'b' is the original data blob (matching the digest), it contains refrerences to the config and layers
		mani, e = mag.Minio.NewManifestDataFromBytes(ctx, b)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to NewManifestData_FromBytes")
			return mani, err
		}
		up.Digest = digest.Digest(mani.Digest)
		up.Size = blob.Size
		e = mag.Minio.PutManifest(ctx, b, up)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to PutManifest")
			return mani, err
		}
		// TODO: Add Upstream with digest to graph
		u := types.Upstream{
			Reg:    up.Reg,
			Repo:   up.Repo,
			Tag:    up.Tag,
			Digest: up.Digest,
		}
		upID, e := mag.NeoInsertUpstream(ctx, u)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to NeoInsertUpstream")
			return mani, err
		}
		_ = upID
		mani.Data = b
	}
	mani.Upstream = types.Upstream{
		Reg:    up.Reg,
		Repo:   up.Repo,
		Tag:    up.Tag,
		Digest: up.Digest,
	}
	return
}

func (mag Magazine) ManifestData_Insert(ctx context.Context, m types.ManifestData) (maniID int64, err error) {
	ctx, span := tracer.Start(ctx, "ManifestData_Insert")
	defer span.End()
	if m.Digest == m.Config.Digest {
		logrus.Fatalf("NeoInsertManifest: Manifest.Digest and Manifest.Config.Digest are equal - something went wrong\n Digest:    %s\n ConfigDig: %s ", m.Digest, m.Config.Digest)
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
	cfg, v1Img, found, e := mag.Config_Get_ByManifest(ctx, req, ociMani)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to mag.Config_Get_ByManifest")
		return
	}
	cfg.MediaType = ociMani.Config.MediaType
	if !found {
		err = utils.NewError(l, span, "failed to find config for manifest")
		return
	}
	//logrus.Infof("Found Config: ID:%d #Labels:%d", cfg.ID, len(cfg.Labels.Map))
	md.Config = cfg
	// Link Manifest to Config
	utils.Debug(l, span, "Linking Manifest to Config: '%d' -> '%d'", md.ID, cfg.ID)
	e = mag.Database.ManifestData_LinkToConfig(ctx, md.ID, cfg)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to mag.LinkManifestToConfig")
		return
	}
	/// Extract Platform (Will be put in by Config_GetInsert_ByOCIManifest in case it is embedded)
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
	return
}

func (mag Magazine) ManifestData_Get_ByID(ctx context.Context, mdID int64) (md types.ManifestData, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestData_Get_ByID",
		"maniDataID": mdID,
	})
	defer span.End()
	md, mdFound, e := mag.Database.ManifestData_Get_ByID(ctx, mdID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to ManifestData_Get_ByID")
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
