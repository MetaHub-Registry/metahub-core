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
	colID, colTagID, success, e := mag.Database.CollectionGetIDByYaml(ctx, col)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to CollectionGetIDByYaml")
		return
	}
	if !success {
		colID, colTagID, e = mag.Database.CollectionCreateByYaml(ctx, col)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to CollectionCreateByYaml")
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

// BindManifestToProfile authoritatively binds a manifest to the profile
// named by a collection entry's explicit `profile:` field. It mirrors
// the generic Slot/profile wiring the parse path applies for the
// default profile (Slot HAS_PUSHED + PULLED_BY + rule unwind +
// deterministic-annotation injection), so the manifest becomes servable
// under that profile.
//
// An unresolved profile label is a hard error: a mis-authored
// collection should surface loudly rather than silently binding
// nothing. An empty label is a no-op (caller falls back to the existing
// image-annotation / generic path).
func (mag Magazine) BindManifestToProfile(ctx context.Context, repo, tag, profileLabel string, maniDataID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgName,
		"func":    "BindManifestToProfile",
		"repo":    repo,
		"tag":     tag,
		"profile": profileLabel,
	})
	defer span.End()
	if profileLabel == "" {
		return nil
	}
	prf, found, e := mag.Database.ProfileGetByLabel(ctx, profileLabel)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to resolve collection profile '%s'", profileLabel)
	}
	if !found {
		return utils.NewError(l, span, "collection profile '%s' does not resolve to an existing profile", profileLabel)
	}
	slot, e := mag.Database.SlotGetCreate(ctx, repo, tag, profileLabel)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "SlotGetCreate(%s:%s#%s)", repo, tag, profileLabel)
	}
	// Collection profile-pin binding, not a user push → no user attribution ("").
	if e = mag.Database.SlotAppendPushedManifest(ctx, slot.ID, maniDataID, ""); e != nil {
		return utils.ErrorHandler(l, span, e, "SlotAppendPushedManifest(slot=%d, manifest=%d)", slot.ID, maniDataID)
	}
	if e = mag.Database.LinkManifestToProfile(ctx, maniDataID, prf.ID, "PULLED_BY"); e != nil {
		return utils.ErrorHandler(l, span, e, "LinkManifestToProfile(manifest=%d, profile=%d)", maniDataID, prf.ID)
	}
	if e = mag.Database.ProfileUnwindRules(ctx, prf.ID, maniDataID); e != nil {
		return utils.ErrorHandler(l, span, e, "ProfileUnwindRules(profile=%d, manifest=%d)", prf.ID, maniDataID)
	}
	injected, skipped, e := mag.Database.ProfileInjectDeterministicAnnotations(ctx, prf.ID, maniDataID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "ProfileInjectDeterministicAnnotations(profile=%d, manifest=%d)", prf.ID, maniDataID)
	}
	utils.Info(l, span, "bound manifest %d to profile '%s' (injected=%d skipped=%d)", maniDataID, profileLabel, injected, skipped)
	return nil
}

func (mag Magazine) RemoveCollectionByRequest(ctx context.Context, req types.Request) (found bool, err error) {
	c := types.NewCollection(req.Repo, req.Tag)
	return mag.Database.CollectionRemove(ctx, c)
}

// HeadManifestNotFound is called when a manifest HEAD by tag finds no local
// manifest. It discovers the image from configured upstream registries.
// The optional targetReg parameter, when provided, restricts discovery to a
// single upstream registry (used when the request path includes a registry prefix).
func (mag Magazine) HeadManifestNotFound(ctx context.Context, req types.Request, targetReg ...string) (md string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgName,
		"func":    "mag.HeadManifestNotFound",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
	})
	defer span.End()
	_, colFound, e := mag.Database.CollectionGetByRepoTagWoManifests(ctx, req.Repo, req.Tag)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, colErrFrmt, "collection", req.Repo, req.Tag)
		return
	}
	if !colFound {
		err = utils.NewError(l, span, "collection for repo '%s' and tag '%s' not found", req.Repo, req.Tag)
		return
	}
	// Discover Upstream image — forward targetReg if provided
	blob, reg, found, e := mag.DiscoverUpstream(ctx, req.Repo, req.Tag, targetReg...)
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
		e := mag.UpstreamBlobInsertManifest(ctx, req, blob)
		if e != nil {
			err = errors.Wrap(e, "failed to insert upstreamblob collection")
			l.Error(err.Error())
			span.RecordError(err)
			return
		}

	case "application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.index.v1+json":
		// Multi-arch index (Docker manifest list / OCI image index): expand it
		// into the supported child manifests instead of rejecting it (D5).
		e := mag.UpstreamBlobInsertIndex(ctx, req, blob)
		if e != nil {
			err = errors.Wrap(e, "failed to insert upstream index collection")
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
