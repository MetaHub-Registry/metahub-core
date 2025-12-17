package magazine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	ociv1 "github.com/regclient/regclient/types/oci/v1"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const pkgNameConfig = "magazine/config.go"

func (mag Magazine) CacheImageConfigViaUpstream(ctx context.Context, up upstream.UpstreamTriple) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameConfig,
		"func":    "CacheImageConfigViaUpstream",
	})
	defer span.End()
	b, _, e := mag.GetBlobByUpstreamTriple(ctx, up)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get blob for %s", up.GetLabel())
		return err
	}
	e = mag.Minio.PutBlob(ctx, b, up.Digest.String())
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to PutConfig")
		return err
	}
	return
}

func (mag Magazine) CachImageConfig(ctx context.Context, mani *types.ManifestData) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameConfig,
		"func":    "CachImageConfig",
		"reg":     mani.Upstream.Reg,
		"repo":    mani.Upstream.Repo,
		"tag":     mani.Upstream.Tag,
		"digest":  utils.ShortenDigest(mani.Config.Digest),
	})
	defer span.End()
	up := upstream.UpstreamTriple{
		Reg:    mani.Upstream.Reg,
		Repo:   mani.Upstream.Repo,
		Tag:    mani.Upstream.Tag,
		Digest: digest.Digest(mani.Config.Digest),
	}
	cfg, isCached, e := mag.Minio.GetConfig(ctx, up.Digest.String())
	_ = cfg
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get config from Minio")
		return
	}
	if !isCached {
		l.Warnf("Config not cached: %s\n%s", up.GetLabel(), up.Digest)
		b, blob, e := mag.GetBlobByUpstreamTriple(ctx, up)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get blob for %s", up.GetLabel())
			return err
		}

		logrus.Warnf("Blob fetched: %v", blob)
		e = mag.Minio.PutBlob(ctx, b, mani.Config.Digest)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to PutConfig")
			return err
		}
		// unmarshal config
		var imgCfg v1.Image
		e = json.Unmarshal(b, &imgCfg)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to unmarshal config")
			return err
		}
		mani.Platform.Arch = imgCfg.Architecture
		mani.Platform.OS = imgCfg.OS
		mani.Platform.Variant = imgCfg.Variant
	}
	return
}

func (mag Magazine) InsertConfig(ctx context.Context, cfg types.Config, up types.Upstream) (cfgID int64, err error) {
	return mag.Database.Config_Insert(ctx, cfg)

}

func (mag Magazine) ConfigBlob_GetByHTTPRequest(ctx context.Context, r *http.Request) (cfg types.Config, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "magazine/config.go",
		"func":    "ConfigBlob_GetByHTTPRequest",
	})
	defer span.End()
	_ = l
	req := types.NewRequest(ctx, r)
	span.AddEvent("req", trace.WithAttributes(
		attribute.String("ContentType", req.ContentType),
		attribute.String("Digest", req.Digest),
		attribute.String("Method", req.Method),
		attribute.String("Host", req.Host),
		attribute.String("Repo", req.Repo),
		attribute.String("Tag", req.Tag),
		attribute.String("Scope", req.Scope),
		attribute.String("Repo", req.Repo),
		attribute.String("Tag", req.Tag),
		attribute.String("User", req.User),
		attribute.String("UserRole", req.UserRole),
		attribute.String("Profile", req.Profile),
		attribute.String("Length", fmt.Sprintf("%d", req.Length)),
		attribute.String("Range", req.Range),
		attribute.String("UUID", req.UUID),
	))
	return mag.Minio.GetTypesConfig(ctx, req.Digest)
}

// Take in a manifest with a config DIGEST and return the config from Blob Storage
func (mag Magazine) Config_Get_ByManifest(ctx context.Context, req types.Request, oMani types.OCIManifest) (cfg types.Config, v1Img v1.Image, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameConfig,
		"func":    "mag.Config_Get_ByManifest",
		"host":    req.Host,
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  cfg.Digest,
	})
	defer span.End()
	label := fmt.Sprintf("%s/%s:%s", req.Host, req.Repo, req.Tag)
	cfg, cfgFound, e := mag.Config_GetInsert_ByOCIManifest(ctx, oMani, label)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to Config.GetCreateByRequest")
		return
	}
	if !cfgFound {
		utils.Debug(l, span, "Config '%s' not found '%s'. Insert with ID %d ", utils.ShortenDigest(cfg.Digest), label, cfg.ID)
	} else {
		utils.Debug(l, span, "Config '%s' found: %s", utils.ShortenDigest(cfg.Digest), label)
	}
	v1Img, v1Found, e := mag.Minio.GetConfig(ctx, cfg.Digest)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get Config from Minio")
		return
	}
	found = v1Found
	return
}

func (mag Magazine) Config_GetByID(ctx context.Context, cfgID int64) (cfg types.Config, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "magazine/config.go",
		"func":    "mag.Config_GetByID",
	})
	defer span.End()
	cfg, found, err = mag.Database.Config_GetByID(ctx, cfgID)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get Config by ID from graph")
		return
	}
	if !found {
		utils.Debug(l, span, "Config not found")
		return
	}
	cfg, err = mag.Minio.ConfigGetObj(ctx, cfg.Digest)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get Config from Minio")
		return
	}
	return cfg, true, nil
}

func (mag Magazine) StoreConfigUpload(ctx context.Context, bu *types.OCIBlobUpload, buf []byte) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameConfig,
		"func":    "mag.StoreConfigUpload",
		"buID":    bu.ID,
		"digest":  bu.Digest,
	})
	defer span.End()
	// NOTE: Wait; we are getting the actual config, not only the descriptor
	// Store Blob
	e := mag.Minio.PutBlob(ctx, buf, bu.Digest)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to store config in Minio")
		return
	}
	c := ociv1.Image{
		Config: ociv1.ImageConfig{
			Labels: map[string]string{},
		},
	}
	// Insert Config into Backend
	cfg := types.Config{
		Digest:    bu.Digest,
		MediaType: v1.MediaTypeImageConfig,
		Size:      int64(len(buf)),
		Data:      buf,
		Cfg:       c,
	}
	utils.Debug(l, span, "Store config upload without knowing which mediaType it actually is")
	/*
		switch bu.MediaType {
		case "application/vnd.docker.container.image.v1+json":
			var imgCfg v1.Image
			err = json.Unmarshal(buf, &imgCfg)
			if err != nil {
				logrus.Error(err)
				return
			}
			cfg.Cfg.Config.Labels = imgCfg.Config.Labels
			//anno, err := types.NewAnnotationsFromMap(imgCfg.Config.Labels)
			//if err != nil {
			//	logrus.Error(err)
			//	return err
			//}
			//cfg.Labels = anno

		case "application/vnd.oci.image.config.v1+json":
			var imgCfg ociv1.Image
			err = json.Unmarshal(buf, &imgCfg)
			if err != nil {
				logrus.Error(err)
				return
			}
			//	anno, err := types.NewAnnotationsFromMap(imgCfg.Config.Labels)
			//	if err != nil {
			//		logrus.Error(err)
			//		return err
			//	}
			//	cfg.Cfg.Config.Labels = imgCfg.Config.Labels
			//	cfg.Labels = anno
		case "":
			// OCI conformance test does not set the media type
			bu.MediaType = "application/vnd.oci.image.config.v1+json"
			var imgCfg ociv1.Image
			err = json.Unmarshal(buf, &imgCfg)
			if err != nil {
				logrus.Error(err)
				return
			}
			//	anno, err := types.NewAnnotationsFromMap(imgCfg.Config.Labels)
			//	if err != nil {
			//		logrus.Error(err)
			//		return err
			//	}
			//	cfg.Cfg.Config.Labels = imgCfg.Config.Labels
			//	cfg.Labels = anno
		default:
			err := errors.Errorf("unknown media type: %s", bu.MediaType)
			logrus.Error(err)
			return err
		}
	*/
	utils.Debug(l, span, "What if config is already there?")

	_, e = mag.Database.Config_Insert(ctx, cfg)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to insert Config into graph")
		return
	}
	e = mag.Minio.BlobPutObj(ctx, bu.Digest, buf)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to store Config in Minio")
	}
	return
}

func (mag Magazine) Config_GetInsert_ByOCIManifest(ctx context.Context, ociMani types.OCIManifest, label string) (cfg types.Config, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameConfig,
		"func":    "mag.Config_GetInsert_ByOCIManifest",
		"label":   label,
		"digest":  ociMani.Config.Digest.String(),
	})
	defer span.End()
	span.AddEvent("debug", trace.WithAttributes(
		attribute.KeyValue{Key: "oci.manifest.config.digest", Value: attribute.StringValue(ociMani.Config.Digest.String())},
		attribute.KeyValue{Key: "ociMani.Config.Data.length", Value: attribute.IntValue(len(ociMani.Config.Data))},
	))
	/// OCI Manifest might embed the config as Data
	if len(ociMani.Config.Data) > 0 {
		utils.Debug(l, span, "Config embedded in OCI Manifest")
		dig := digest.FromBytes(ociMani.Config.Data)
		if dig.String() != ociMani.Config.Digest.String() {
			err = utils.NewError(l, span, "digest mismatch: %s != %s", dig, ociMani.Config.Digest)
			return
		}
		bu := types.OCIBlobUpload{
			Digest:    ociMani.Config.Digest.String(),
			Data:      ociMani.Config.Data,
			MediaType: ociMani.Config.MediaType,
		}
		err = mag.StoreConfigUpload(ctx, &bu, ociMani.Config.Data)
		if err != nil {
			logrus.Error(err)
			return
		}
		utils.Warn(l, span, "Finished upload of embedded config")
	} else {
		utils.Debug(l, span, "Config NOT embedded in OCI Manifest - fetch from Blob Storage")
	}
	cfg, found, err = mag.Database.Config_GetInsert_ByOCIManifest(ctx, ociMani, label)
	return
}

func (mag Magazine) Config_AddLabel(ctx context.Context, cfgID int64, a types.Annotation) (err error) {
	return mag.Database.Config_AddLabel(ctx, cfgID, a)
}

func (mag Magazine) Configs_DeleteUnused(ctx context.Context) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameConfig,
		"func":    "mag.Configs_DeleteUnused",
	})
	defer span.End()
	e := mag.Database.Configs_DeleteUnused(ctx)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to delete unused layers")
		return
	}
	e = mag.Database.BlobUpload_DeleteUnused(ctx)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to delete unused blobUploads")
		return
	}
	return
}
