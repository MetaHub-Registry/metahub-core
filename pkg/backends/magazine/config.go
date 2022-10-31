package magazine

import (
	"context"
	"encoding/json"
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

func (mag Magazine) NeoInsertConfig(ctx context.Context, cfg types.Config, up types.Upstream) (cfgID int64, err error) {
	cfgID, success, err := mag.Neo.GetConfig(cfg)
	if err != nil {
		return
	}
	if !success {
		cfgID, err = mag.Neo.CreateConfig(cfg, up)
		if err != nil {
			return
		}
	}
	return
}

func (mag Magazine) ConfigBlob_GetByHTTPRequest(r *http.Request) (cfg types.Config, found bool, err error) {
	ctx := context.Background()
	req := types.NewRequest(r)
	return mag.Minio.GetTypesConfig(ctx, req.Digest)
}

func (mag Magazine) StoreConfigUpload(ctx context.Context, bu *types.OCIBlobUpload, buf []byte) (err error) {
	// TODO: Wait; we are getting the actual config, not only the descriptor

	// Store Blob
	err = mag.Minio.PutConfig(ctx, buf, bu.Digest)
	if err != nil {
		logrus.Error(err)
		return
	}
	// Insert Layer into SQLite
	l := types.Config{
		Digest:    bu.Digest,
		MediaType: "application/vnd.docker.container.image.v1+json",
		Size:      int64(len(buf)),
	}
	err = mag.SQlite.Config.Upsert(&l)
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}
