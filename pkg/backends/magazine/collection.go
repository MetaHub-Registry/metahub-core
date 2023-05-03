package magazine

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgName    = "magazine/collection.go"
	colErrFrmt = "failed to get %s for '%s:%s'"
)

func (mag Magazine) CreateCollection(ctx context.Context, col yamlCfg.ManifestCollection) (colID, colTagID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgName,
		"func":    "CreateCollection",
		"repo":    col.Name,
		"tag":     col.Tag,
	})
	defer span.End()
	colID, colTagID, success, e := mag.Database.Collection_GetID_ByYaml(ctx, col)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to Collection_GetID_ByYaml")
		return
	}
	if !success {
		colID, colTagID, e = mag.Database.Collection_Create_ByYaml(ctx, col)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to Collection_Create_ByYaml")
			return
		}
	}
	utils.Debug(l, span, "colID:%d / colTagID:%d", colID, colTagID)
	return
}

func (mag Magazine) NeoLinkCollectionToManifest(ctx context.Context, colID, colTagID, maniID int64) (err error) {
	err = mag.Database.LinkCollectionToManifest(ctx, colID, maniID)
	if err != nil {
		return
	}
	err = mag.Database.LinkCollectionTagToManifest(ctx, colTagID, maniID)
	return
}

func (mag Magazine) RemoveCollectionByRequest(req types.Request) (found bool, err error) {
	ctx := context.Background()
	c := types.Collection{
		Repo: req.Repo,
		Tag:  req.Tag,
	}
	return mag.Database.Collection_Remove(ctx, c)
}

func (mag Magazine) HeadManistNotFound_ForProfile(ctx context.Context, req types.Request) (md string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgName,
		"func":    "mag.HeadManistNotFound_ForProfile",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
	})
	defer span.End()
	_ = l
	_ = ctx
	//// 1. Find the PULLED upstream Manifest & ManifestData
	mani, maniFound, e := mag.Database.Manifest_Get_ByRepoTag(ctx, req.Repo, req.Tag)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, colErrFrmt, "manifest", req.Repo, req.Tag)
		return
	}
	if !maniFound {
		err = utils.NewError(l, span, "manifest for repo '%s' and tag '%s' not found", req.Repo, req.Tag)
		return md, false, err
	}
	utils.Debug(l, span, "found manifest %s", mani.Upstream.String())
	maniData, maniDataFound, e := mag.Database.ManifestData_GetPulled_ByRepoTag(ctx, req.Repo, req.Tag)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, colErrFrmt, "manifestData", req.Repo, req.Tag)
		return
	}
	if !maniDataFound {
		err = utils.NewError(l, span, "manifestData for repo '%s' and tag '%s' not found", req.Repo, req.Tag)
		return md, false, err
	}
	utils.Debug(l, span, "found manifestData: %s", utils.ShortenDigest(maniData.Digest))
	//// 2. Create a new ManifestData for the profile
	prf, prfFound, e := mag.Database.Profile_Get_ByLabel(ctx, req.Profile)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "error getting profile '%s'", req.Profile)
		return md, false, err
	}
	if !prfFound {
		err = utils.NewError(l, span, "profile '%s' not found", req.Profile)
		return md, false, err
	}
	_ = prf
	// TODO: Create OCI ManifestData for profile

	ociManiDataID, _, e := mag.CreateOCIv1_PulledByPrf(ctx, mani, maniData, prf)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "error creating oci manifest for profile '%s'", req.Profile)
		return md, false, err
	}
	utils.Debug(l, span, "created oci manifest for profile '%s': %d", req.Profile, ociManiDataID)

	//// 3. UnWind the profile for that ManifestData
	//// 4. Compute ManifestList for the profile

	return md, found, err
}
func (mag Magazine) HeadManifestNotFound(ctx context.Context, req types.Request) (md string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgName,
		"func":    "mag.HeadManifestNotFound",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
	})
	defer span.End()
	_, colFound, e := mag.Database.Collection_Get_ByRepoTag_woManifests(ctx, req.Repo, req.Tag)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, colErrFrmt, "collection", req.Repo, req.Tag)
		return
	}
	if !colFound {
		err = utils.NewError(l, span, "collection for repo '%s' and tag '%s' not found", req.Repo, req.Tag)
		return
	}
	// DONE: Discover Upstream image
	blob, reg, found, e := mag.DiscoverUpstream(ctx, req.Repo, req.Tag)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to discover upstream")
		return
	}
	md = blob.MediaType
	if !found {
		utils.Info(l, span, "upstream '%s:%s' not found'", req.Repo, req.Tag)
		found = false
		return
	}
	req.Host = reg
	utils.Info(l, span, "upstream found in registry '%s'. MediaType: %s", reg, md)
	switch md {
	case "application/vnd.docker.distribution.manifest.v2+json":
		// Insert into DB
		e := mag.UpstreamBlob_InsertManifest(ctx, req, blob)
		if e != nil {
			err = errors.Wrap(e, "failed to insert upstreamblob collection")
			l.Error(err.Error())
			span.RecordError(err)
			return
		}

	case "application/vnd.docker.distribution.manifest.list.v2+json":
		e := mag.UpstreamBlob_InsertIndex(ctx, req, blob)
		if e != nil {
			err = errors.Wrap(e, "failed to insert upstreamblob collection")
			l.Error(err.Error())
			span.RecordError(err)
			return
		}

	default:
		err = fmt.Errorf("unsuported media type: %s", md)
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	return
}
