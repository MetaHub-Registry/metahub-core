package magazine

import (
	"context"
	"fmt"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameInternal = "magazine/internal.go"
)

// getMani builds a leaf-keyed types.ManifestWrapper shim for the content-leaf
// behind "<label>#id". Phase 4 (deprecate-manifest-wrapper): :ManifestWrapper
// nodes are gone, so the test helper hydrates the content-leaf
// (label aliases the content-leaf id) and wraps it — mirroring what
// ComputeManifestForLeaf does internally.
func (mag Magazine) getMani(ctx context.Context, br utils.BootstrapResult, label string) (mani types.ManifestWrapper, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameInternal,
		"func":    "getMani",
	})
	defer span.End()
	_ = l
	maniID, e := br.GetInt64(fmt.Sprintf("%s#id", label))
	if e != nil {
		return mani, e
	}
	md, maniFound, err := mag.Database.ManifestGetByID(ctx, maniID)
	if err != nil {
		return mani, err
	}
	if !maniFound {
		return mani, fmt.Errorf("did not find manifest with ID %d", maniID)
	}
	annos := md.Annotations
	if annos.Data == nil {
		annos = types.NewAnnotations()
	}
	if md.Type == "" {
		md.Type = "ORIGINAL"
	}
	mani = types.ManifestWrapper{
		ID:          md.ID,
		Upstream:    md.Upstream,
		Annotations: annos,
		Manifests:   []types.Manifest{md},
	}
	return mani, nil
}

func (mag Magazine) getProfile(ctx context.Context, br utils.BootstrapResult, label string) (prf types.Profile, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameInternal,
		"func":    "getProfile",
	})
	defer span.End()
	_ = l
	_ = br
	prf, prfFound, err := mag.Database.ProfileGetByLabel(ctx, label)
	if err != nil {
		return prf, err
	}
	if !prfFound {
		return prf, fmt.Errorf("did not find profile with label: %s", label)
	}
	return
}

func (mag Magazine) getManiData(ctx context.Context, br utils.BootstrapResult, label string) (mani types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameInternal,
		"func":    "getManiData",
	})
	defer span.End()
	_ = l
	mdID, e := br.GetInt64(fmt.Sprintf("%s#id", label))
	if e != nil {
		return mani, e
	}
	mani, maniFound, err := mag.ManifestGetByID(ctx, mdID)
	if err != nil {
		return mani, err
	}
	if !maniFound {
		return mani, fmt.Errorf("did not find Manifest with ID %d", mdID)
	}
	return mani, nil
}
