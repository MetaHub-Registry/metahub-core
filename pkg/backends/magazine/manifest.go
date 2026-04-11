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
	ycfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

const (
	pkgNameMani = "magazine/manifest.go"
)

// Path Z Phase C3: NeoLinkManifestToUpstream, LinkManifestToManifest,
// and NeoLinkManifestToLayer are deleted — they were thin delegations to
// wrapper-era functions. Content-leaf linking uses :ORIGINAL edges created
// by createManifest, and Slot-based attribution replaces Collection links.

func (mag Magazine) NeoLinkManifestToConfig(ctx context.Context, maniID int64, cfg types.Config) (err error) {
	return mag.Database.ManifestLinkToConfig(ctx, maniID, cfg)
}

func (mag Magazine) NeoLinkManifestToLayer(ctx context.Context, maniID int64, l types.Layer, idx int) (err error) {
	return mag.Database.ManifestLinkToLayer(ctx, maniID, l, idx)
}

func (mag Magazine) NeoLinkManifestToPlatform(ctx context.Context, maniID, platID int64) (err error) {
	return mag.Database.LinkManifestToPlatform(ctx, maniID, platID)
}

func (mag Magazine) GetManifest(ctx context.Context, m ycfg.Manifest) (mani types.Manifest, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameMani,
		"func":    "GetManifest",
	})
	_, span := tracer.Start(ctx, "GetManifest")
	defer span.End()
	up, err := m.ToUpstreamTriple()
	if err != nil {
		logrus.Error(err)
		return
	}
	l = l.WithFields(logrus.Fields{
		"reg":    up.Reg,
		"repo":   up.Repo,
		"tag":    up.Tag,
		"digest": up.Digest,
	})
	req := types.Request{
		Host: up.Reg,
		Repo: up.Repo,
		Tag:  up.Tag,
	}
	// 2023-04-05 check for upstream in graph
	u, found, e := mag.Database.UpstreamGetByRequest(ctx, req)
	if e != nil {
		err = errors.Wrap(e, "failed to get upstream")
		l.Error(err)
		return
	}
	up.Digest = u.Digest
	if !found {
		err = errors.New("upstream not found")
		l.Error(err)
		return
	}
	// The Manifest cached has a Data filed with the original data blob (matching the digest)
	// in addition it has fields populated with the parsed data.
	mani, isCached, err := mag.Minio.GetCachedManifest(ctx, up.Digest)
	if err != nil {
		l.Error(err)
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

func (mag Magazine) ManifestGetByHTTPRequest(ctx context.Context, r *http.Request) (m types.Manifest, found bool, err error) {
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
		"func":    "ManifestDeleteByHTTPRequest",
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
	found, e := mag.RemoveManifestByRequest(ctx, req)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to delete manifest")
		return
	}
	if !found || req.Tag != "" {
		//Not a plain Manifest, let's try to delete a Repo
		logrus.Warn("Not a plain Manifest, let's try to delete a Repo")
		found, e := mag.RemoveCollectionByRequest(ctx, req)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to delete collection")
			return
		}
		if !found {
			err = utils.NewError(l, span, "Repo not found")
			return
		}
		return
	}
	return
}

func (mag Magazine) ManifestGetByRequest(ctx context.Context, req types.Request) (m types.Manifest, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestGetByRequest",
		"repo":    req.Repo,
		"profile": req.Profile,
	})
	defer span.End()
	// Fetch Manifest from SQLite
	m, found, e := mag.Database.ManifestGetTypesByRequest(ctx, req)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get Manifest from Database")
		return
	}
	if !found {
		return m, found, nil
	}
	m, e = mag.Minio.ManifestGetObj(ctx, m.Digest)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get Manifest from Minio")
		return
	}
	return
}

// storeManifestUploadOCIManifest unmarshals the OCI manifest from the raw
// push body, stores it in Minio, inserts layers, de-duplicates by profile,
// and returns the content-leaf types.Manifest.
//
// Path Z Phase C3: all legacy wrapper operations (:ManifestWrapper node creation
// via ManifestGetInsert, Collection linking, ManifestWrapperLinkToUpstream,
// LinkManifestToManifest) are removed. The content-leaf :Manifest
// node is the sole persistent artefact; Slot-based attribution and the
// :ORIGINAL edge to :Upstream handle grouping and provenance.
func (mag Magazine) storeManifestUploadOCIManifest(ctx context.Context, req types.Request, buf []byte) (md types.Manifest, ociMani types.OCIManifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "storeManifestUploadOCIManifest",
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
		return md, ociMani, err
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
	// Phase C3: wrapper :ManifestWrapper node creation fully removed.
	// The Upstream node is created by ManifestGetInsert→createManifest
	// via the :ORIGINAL edge. Slot-based attribution is handled by
	// wireSlotOnPush in the caller (StoreManifestUpload).
	up := types.Upstream{
		Reg:    req.Host,
		Repo:   req.Repo,
		Tag:    req.Tag,
		Digest: dig,
	}
	// Ensure Upstream node exists (for :ORIGINAL edge in createManifest)
	_, _, e = mag.Database.UpstreamGetInsertByRequest(ctx, types.Request{
		Host:   req.Host,
		Repo:   req.Repo,
		Tag:    req.Tag,
		Digest: dig.String(),
	})
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to ensure Upstream exists")
		return
	}
	// Insert layers
	lyrs := []types.Layer{}
	for idx, lyrDesc := range ociMani.Layers {
		utils.Debug(l, span, "Layer %d: %s", idx, utils.ShortenDigest(lyrDesc.Digest.String()))
		lyr, e := types.LayerFromDesc(ctx, lyrDesc)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to create Layer from distribution.Descriptor")
			return
		}
		lyr.ID, e = mag.Database.LayerPlainInsert(ctx, lyr)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to insert layer")
			return
		}
		lyrs = append(lyrs, lyr)
	}
	// Profile lookup
	prf, found, e := mag.Database.ProfileGetByLabel(ctx, req.Profile)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get profile '%s' from database", prf.Label)
		return
	}
	if !found {
		err = utils.NewError(l, span, "profile '%s' not found", prf.Label)
		return
	}
	// De-duplicate: check if a manifest was already pushed for this profile
	md, found, e = mag.Database.ManifestGetPushedByProfile(ctx, up, prf)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get Manifest for profile '%s' from database", prf.Label)
		return
	}
	if found {
		utils.Debug(l, span, "manifest '%s' already pushed for profile: '%s'", up.String(), prf.Label)
		if dig.String() != md.Digest {
			utils.Debug(l, span, "incoming manifest ('%s') is different from already present manifest ('%s')", utils.ShortenDigest(dig.String()), utils.ShortenDigest(md.Digest))
			e = mag.Database.ManifestDelete(ctx, md)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "failed to delete Manifest from Database")
				return
			}
		} else {
			utils.Debug(l, span, "incoming manifest ('%s') is the same as already present manifest ('%s')", utils.ShortenDigest(dig.String()), utils.ShortenDigest(md.Digest))
		}
	}

	// Store content-leaf Manifest
	annos := types.NewAnnotations()
	for k, v := range ociMani.Annotations {
		annos.AddNewAnnotation(k, v)
	}
	md = types.Manifest{
		Digest:       dig.String(),
		MediaType:    ociMani.MediaType,
		ArtifactType: ociMani.ArtifactType,
		Data:         buf,
		Size:         int64(len(buf)),
		Layers:       lyrs,
		Upstream:     up,
		Annotations:  annos,
	}
	e = mag.Database.ManifestGetInsert(ctx, &md)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to getInsert Manifest from Database")
		return
	}
	if e = mag.Database.LinkManifestToProfile(ctx, md.ID, prf.ID, "UPLOADED_BY"); e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to link Manifest to Profile")
		return
	}
	if e = mag.Database.ManifestAddAnnotations(ctx, md.ID, &md.Annotations); e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to add annotations to Manifest")
		return
	}
	// Phase C3: wrapper links (LinkManifestToManifest, ManifestWrapperLinkToUpstream)
	// fully removed. Content-leaf is linked to Upstream via :ORIGINAL edge
	// in createManifest. Slot attribution via wireSlotOnPush in caller.
	return
}

func (mag Magazine) RemoveManifestByRequest(ctx context.Context, req types.Request) (found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "RemoveManifestByRequest",
		"reg":     req.Host,
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
		"digest":  req.Digest,
	})
	defer span.End()
	m := types.Manifest{
		Upstream: types.Upstream{
			Repo: req.Repo,
			Tag:  req.Tag,
		},
		Digest: req.Digest,
	}

	found, err = mag.Database.ManifestRemove(ctx, req)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to Database.ManifestRemove")
		return
	}
	if found {
		// FOUND: Remove Manifest Blob
		e := mag.Minio.ManifestRemoveObj(ctx, m.Digest)
		if e != nil {
			utils.ErrorHandler(l, span, e, "while in RemoveManifestByRequest (we won't fail here tho)")
			return
		}
	}
	return
}
