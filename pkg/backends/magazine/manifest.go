package magazine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameMani = "magazine/manifest.go"
)

func (mag Magazine) NeoLinkManifestToUpstream(ctx context.Context, maniID, upID int64) (err error) {
	return mag.Database.Manifest_LinkToUpstream(ctx, maniID, upID)
}

func (mag Magazine) LinkManifestToManifestData(ctx context.Context, maniID, maniDataID int64, props ...string) (err error) {
	return mag.Database.LinkManifestToManifestData(ctx, maniID, maniDataID, props...)
}

func (mag Magazine) NeoLinkManifestDataToConfig(ctx context.Context, maniID int64, cfg types.Config) (err error) {
	return mag.Database.ManifestData_LinkToConfig(ctx, maniID, cfg)
}

func (mag Magazine) NeoLinkManifestToLayer(ctx context.Context, maniID int64, l types.Layer, idx int) (err error) {
	return mag.Database.ManifestData_LinkToLayer(ctx, maniID, l, idx)
}

func (mag Magazine) NeoLinkManifestToPlatform(ctx context.Context, maniID, platID int64) (err error) {
	return mag.Database.LinkManifestToPlatform(ctx, maniID, platID)
}

func (mag Magazine) ManifestGetByHTTPRequest(ctx context.Context, r *http.Request) (m types.ManifestData, found bool, err error) {
	req := types.NewRequest(ctx, r)
	repo := chi.URLParam(r, "repo")
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestGetByHTTPRequest",
		"method":  r.Method,
		"uri":     r.RequestURI,
		"repo":    repo,
		"profile": req.Profile,
	})
	defer span.End()
	if req.Profile == "" {
		utils.Debug(l, span, "ManifestGetByHTTPRequest() - no profile set")
		return
	}

	msg := fmt.Sprintf("%-5s ManifestHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	if req.Profile != "generic" {
		msg += fmt.Sprintf(" - profile: %s", req.Profile)
	}
	utils.Debug(l, span, msg)
	m, found, e := mag.ManifestGetByRequest(ctx, req)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get manifest")
		return
	}
	return
}

func (mag Magazine) ManifestDeleteByHTTPRequest(ctx context.Context, r *http.Request) (err error) {
	req := types.NewRequest(ctx, r)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "Manifest_DeleteByHTTPRequest",
		"method":  r.Method,
		"uri":     r.RequestURI,
		"repo":    req.Repo,
		"profile": req.Profile,
	})
	defer span.End()
	_ = ctx
	msg := fmt.Sprintf("%-5s DeleteManifestHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	utils.Info(l, span, msg)
	found, e := mag.RemoveManifestByRequest(req)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to delete manifest")
		return
	}
	if !found || req.Tag != "" {
		//Not a plain Manifest, let's try to delete a Collection
		logrus.Warn("Not a plain Manifest, let's try to delete a Collection")
		found, e := mag.RemoveCollectionByRequest(req)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to delete collection")
			return
		}
		if !found {
			err = utils.NewError(l, span, "Collection not found")
			return
		}
		return
	}
	return
}

func (mag Magazine) ManifestGetByRequest(ctx context.Context, req types.Request) (m types.ManifestData, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestGetByRequest",
		"repo":    req.Repo,
		"profile": req.Profile,
	})
	defer span.End()
	// Fetch Manifest from SQLite
	m, found, e := mag.Database.ManifestDataGetTypesByRequest(ctx, req)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get Manifest from Database")
		return
	}
	if !found {
		err = utils.NewError(l, span, "manifestData not found")
		return
	}
	m, e = mag.Minio.ManifestGetObj(ctx, m.Digest)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get Manifest from Minio")
		return
	}
	return
}

// storeManifestUpload_OCIManifest
func (mag Magazine) storeManifestUpload_OCIManifest(ctx context.Context, req types.Request, buf []byte) (mani types.Manifest, ociMani types.OCIManifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "storeManifestUpload_OCIManifest",
		"reg":     req.Host,
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
	})
	defer span.End()
	ociMani = types.OCIManifest{
		MediaType: req.ContentType,
	}
	e := json.Unmarshal(buf, &ociMani)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to unmarshal OCI Manifest '%s'", string(buf))
		return mani, ociMani, err
	}
	utils.Debug(l, span, "unmarshal ociMani: %v", ociMani)
	dig := digest.FromBytes(buf)
	req.Digest = dig.String()
	if req.Digest != "" && req.Digest != dig.String() {
		err = utils.NewError(l, span, "digest mismatch: %s != %s", req.Digest, dig.String())
		return
	}
	e = mag.Minio.ManifestsPutObj(ctx, dig.String(), buf)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to put Manifest into Minio")
		return
	}
	col, _, e := mag.Database.Collection_GetCreate_ByRepoTag(ctx, req.Repo, req.Tag)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get Collection from Database")
		return
	}

	// Store Manifest (the standin under which all ManifestData are stored)
	up := types.Upstream{
		Reg:    req.Host,
		Repo:   req.Repo,
		Tag:    req.Tag,
		Digest: dig,
	}
	mani = types.Manifest{
		Upstream:  up,
		Manifests: []types.ManifestData{},
	}

	mID, e := mag.Database.Manifest_GetInsertWithRequest(ctx, mani, req)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get Manifest from Database")
		return
	}
	// Link Manifest to Collection
	e = mag.Database.LinkCollectionToManifest(ctx, col.ID, mID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to LinkCollectionToManifest")
		return
	}
	e = mag.Database.LinkCollectionTagToManifest(ctx, col.ColTag.ID, mID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to LinkCollectionTagToManifest")
		return
	}
	mani.ID = mID
	lyrs := []types.Layer{}
	for idx, lyrDesc := range ociMani.Layers {
		utils.Debug(l, span, "Layer %d: %s", idx, utils.ShortenDigest(lyrDesc.Digest.String()))
		lyrs = append(lyrs, types.Layer{
			Digest:    lyrDesc.Digest.String(),
			MediaType: lyrDesc.MediaType,
			Size:      lyrDesc.Size,
		})
		lyr, e := types.Layer_FromDesc(ctx, lyrDesc)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to create Layer from distribution.Descriptor")
			return
		}
		lyr.ID, e = mag.Database.Layer_PlainInsert(ctx, lyr)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to insert layer")
			return
		}
	}
	prf, found, e := mag.Database.Profile_Get_ByLabel(ctx, req.Profile)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get profile '%s' from database", prf.Label)
		return
	}
	if !found {
		err = utils.NewError(l, span, "profile '%s' not found", prf.Label)
		return
	}
	md, found, e := mag.Database.ManifestData_GetPushed_ByProfile(ctx, up, prf)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get ManifestData for profile '%s' from database", prf.Label)
		return
	}
	if found {
		utils.Debug(l, span, "manifest '%s' already pushed for profile: '%s'", up.String(), prf.Label)
		if dig.String() != md.Digest {
			utils.Debug(l, span, "incoming manifest ('%s') is different from already present manifest ('%s')", utils.ShortenDigest(dig.String()), utils.ShortenDigest(md.Digest))
			// TODO: Remove manifestdata already pushed
			e = mag.Database.ManifestData_Delete(ctx, md)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "failed to delete ManifestData from Database")
				return
			}
		} else {
			utils.Debug(l, span, "incoming manifest ('%s') is the same as already present manifest ('%s')", utils.ShortenDigest(dig.String()), utils.ShortenDigest(md.Digest))
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
	e = mag.Database.ManifestData_GetInsert(ctx, &md)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to getInsert ManifestData from Database")
		return
	}
	mani.Manifests = append(mani.Manifests, md)
	e = mag.LinkManifestToManifestData(ctx, mID, md.ID, "UPLOADED_BY")
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to mag.LinkManifestToManifestData")
		return
	}
	e = mag.Database.LinkManifestDataToProfile(ctx, md.ID, prf.ID, "UPLOADED_BY")
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to mag.LinkManifestToManifestData")
		return
	}
	// Remove orhpaned layers
	e = mag.Layers_DeleteUnused(ctx)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to delete unused layers")
		return
	}
	// Remove orhpaned config
	e = mag.Configs_DeleteUnused(ctx)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to delete unused config")
		return
	}
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
