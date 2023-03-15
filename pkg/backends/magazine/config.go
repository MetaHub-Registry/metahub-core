package magazine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	ociv1 "github.com/regclient/regclient/types/oci/v1"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (mag Magazine) CacheImageConfigViaUpstream(ctx context.Context, up upstream.UpstreamTriple) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "backend/magazine/config.go",
		"func":       "CacheImageConfigViaUpstream",
	})
	b, blob, err := upstream.GetBlobByUpstreamTriple(ctx, up)
	if err != nil {
		err = errors.Wrapf(err, "failed to get blob for %s", up.GetLabel())
		l.Error(err)
		return err
	}
	_ = blob
	err = mag.Minio.PutConfig(ctx, b, up.Digest.String())
	if err != nil {
		l.Error(err)
		return err
	}
	return
}

func (mag Magazine) CachImageConfig(ctx context.Context, mani *types.ManifestData) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "backend/magazine/config.go",
		"func":       "CachImageConfig",
		"reg":        mani.Upstream.Reg,
		"repo":       mani.Upstream.Repo,
		"tag":        mani.Upstream.Tag,
		"digest":     utils.ShortenDigest(mani.Config.Digest),
	})
	up := upstream.UpstreamTriple{
		Reg:    mani.Upstream.Reg,
		Repo:   mani.Upstream.Repo,
		Tag:    mani.Upstream.Tag,
		Digest: digest.Digest(mani.Config.Digest),
	}
	cfg, isCached, err := mag.Minio.GetConfig(ctx, up.Digest.String())
	_ = cfg
	if err != nil {
		logrus.Error(err)
		return
	}
	if !isCached {
		l.Warnf("Config not cached: %s\n%s", up.GetLabel(), up.Digest)
		b, blob, err := upstream.GetBlobByUpstreamTriple(ctx, up)
		if err != nil {
			err = errors.Wrapf(err, "failed to get blob for %s", up.GetLabel())
			logrus.Error(err)
			return err
		}

		logrus.Warnf("Blob fetched: %v", blob)
		err = mag.Minio.PutConfig(ctx, b, mani.Config.Digest)
		if err != nil {
			logrus.Error(err)
			return err
		}
		// unmarshal config
		var imgCfg v1.Image
		err = json.Unmarshal(b, &imgCfg)
		if err != nil {
			logrus.Error(err)
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
	req := types.NewRequest(ctx, r)
	return mag.Minio.GetTypesConfig(ctx, req.Digest)
}

// Take in a manifest with a config DIGEST and return the config from Blob Storage
func (mag Magazine) Config_Get_ByManifest(ctx context.Context, req types.Request, oMani types.OCIManifest) (cfg types.Config, v1Img v1.Image, found bool, err error) {
	ctx, span := tracer.Start(ctx, "Config_Get_ByManifest")
	defer span.End()
	span.SetAttributes(
		attribute.Key("host").String(req.Host),
		attribute.Key("repo").String(req.Repo),
		attribute.Key("tag").String(req.Tag),
	)
	label := fmt.Sprintf("%s/%s:%s", req.Host, req.Repo, req.Tag)
	cfg, cfgFound, err := mag.Config_GetInsert_ByOCIManifest(ctx, oMani, label)
	if err != nil {
		err = errors.Wrap(err, "failed to Config.GetCreateByRequest")
		span.RecordError(err)
		return
	}
	if !cfgFound {
		logrus.Errorf("Config '%s' not found '%s'. Insert with ID %d ", utils.ShortenDigest(cfg.Digest), label, cfg.ID)
	} else {
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", fmt.Sprintf("Config '%s' found: %s", utils.ShortenDigest(cfg.Digest), label)),
		))
	}
	v1Img, v1Found, err := mag.Minio.GetConfig(ctx, oMani.Config.Digest.String())
	if err != nil {
		err = errors.Wrap(err, "failed to get Config from Minio")
		return
	}
	found = v1Found
	//fmt.Printf("v1Img: \n%v\n", v1Img)
	return
}

func (mag Magazine) StoreConfigUpload(ctx context.Context, bu *types.OCIBlobUpload, buf []byte) (err error) {
	// TODO: Wait; we are getting the actual config, not only the descriptor
	ctx, span := tracer.Start(ctx, "StoreConfigUpload")
	defer span.End()
	span.SetAttributes(
		attribute.Key("buID").Int64(bu.ID),
	)
	// Store Blob
	err = mag.Minio.PutConfig(ctx, buf, bu.Digest)
	if err != nil {
		logrus.Error(err)
		span.RecordError(err)
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
		MediaType: "unknown",
		Size:      int64(len(buf)),
		Data:      buf,
		Cfg:       c,
	}
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", "Store config upload without knowing which mediaType it actually is"),
		attribute.String("cfg.digest", utils.ShortenDigest(bu.Digest)),
		attribute.String("cfg.mediaType", bu.MediaType),
	))
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
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "warn"),
		attribute.String("log.message", "What if config is already there?"),
	))
	_, err = mag.Database.Config_Insert(ctx, cfg)
	if err != nil {
		logrus.Error(err)
		span.RecordError(err)
		return
	}
	err = mag.Minio.ConfigPutObj(ctx, bu.Digest, buf)
	return
}

func (mag Magazine) Config_GetInsert_ByOCIManifest(ctx context.Context, ociMani types.OCIManifest, label string) (cfg types.Config, found bool, err error) {
	ctx, span := tracer.Start(ctx, "Config_GetInsert_ByOCIManifest")
	defer span.End()
	/// OCI Manifest might embed the config as Data
	if len(ociMani.Config.Data) > 0 {
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("label", label),
			attribute.String("log.message", "Config embedded in OCI Manifest"),
			attribute.String("config.digest", utils.ShortenDigest(ociMani.Config.Digest.String())),
		))
		dig := digest.FromBytes(ociMani.Config.Data)
		if dig.String() != ociMani.Config.Digest.String() {
			err = fmt.Errorf("digest mismatch: %s != %s", dig, ociMani.Config.Digest)
			logrus.Error(err)
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
		logrus.Warn("Finished upload of embedded config")
	}
	cfg, found, err = mag.Database.Config_GetInsert_ByOCIManifest(ctx, ociMani, label)
	return
}

func (mag Magazine) Config_AddLabel(ctx context.Context, cfgID int64, a types.Annotation) (err error) {
	return mag.Database.Config_AddLabel(ctx, cfgID, a)
}

func (mag Magazine) Configs_DeleteUnused(ctx context.Context) (err error) {
	_, span := tracer.Start(ctx, "Config_DeleteUnused")
	defer span.End()
	err = mag.Database.Configs_DeleteUnused(ctx)
	if err != nil {
		err = errors.Wrapf(err, "failed to delete unused layers")
		return
	}
	err = mag.Database.BlobUpload_DeleteUnused(ctx)
	if err != nil {
		err = errors.Wrapf(err, "failed to delete unused blobUploads")
		return
	}
	return
}
