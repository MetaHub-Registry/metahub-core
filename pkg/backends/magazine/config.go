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
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
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

func (mag Magazine) CachImageConfig(ctx context.Context, mani *types.Manifest) (err error) {
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

func (mag Magazine) ConfigBlob_GetByHTTPRequest(r *http.Request) (cfg types.Config, found bool, err error) {
	ctx := context.Background()
	req := types.NewRequest(r)
	return mag.Minio.GetTypesConfig(ctx, req.Digest)
}

// Take in a manifest with a config DIGEST and return the config from Blob Storage
func (mag Magazine) Config_Get_ByManifest(ctx context.Context, req types.Request, oMani types.OCIManifest) (cfg types.Config, v1Img v1.Image, found bool, err error) {
	label := fmt.Sprintf("%s/%s:%s", req.Host, req.Repo, req.Tag)
	cfg, cfgFound, err := mag.Config_GetInsert_ByOCIManifest(oMani, label)
	if err != nil {
		err = errors.Wrap(err, "failed to Config.GetCreateByRequest")
		return
	}
	if !cfgFound {
		logrus.Errorf("Config '%s' not found: %s - is that a problem?", utils.ShortenDigest(cfg.Digest), label)
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
	// Store Blob
	err = mag.Minio.PutConfig(ctx, buf, bu.Digest)
	if err != nil {
		logrus.Error(err)
		return
	}
	// Insert Config into Backend
	cfg := types.Config{
		Digest:    bu.Digest,
		MediaType: bu.MediaType, //"application/vnd.docker.container.image.v1+json",
		Size:      int64(len(buf)),
		Data:      buf,
	}
	/// WHat if config is already there?
	_, err = mag.Database.Config_Insert(ctx, cfg)
	if err != nil {
		logrus.Error(err)
		return
	}
	err = mag.Minio.ConfigPutObj(ctx, bu.Digest, buf)
	return
}

func (mag Magazine) Config_GetInsert_ByOCIManifest(ociMani types.OCIManifest, label string) (cfg types.Config, found bool, err error) {
	ctx := context.Background()
	/// OCI Manifest might embed the config as Data
	if len(ociMani.Config.Data) > 0 {
		/*
			logrus.Warnf("Config embedded in OCI Manifest: %s", label)
			fmt.Printf("Data: \n%s\n", ociMani.Config.Data)
			dig := digest.FromBytes(ociMani.Config.Data)
			if dig.String() != ociMani.Config.Digest.String() {
				logrus.Errorf("Digest mismatch: %s != %s", dig, ociMani.Config.Digest)
			}
		*/
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
	}
	cfg, found, err = mag.Database.Config_GetInsert_ByOCIManifest(ociMani, label)
	return
}
