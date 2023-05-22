package neo

import (
	"context"
	"fmt"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameInternal = "neo/internal.go"
)

func (n Neo) getMani(ctx context.Context, br utils.BootstrapResult, label string) (mani types.Manifest, err error) {
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
	mani, maniFound, err := n.Manifest_GetByID(ctx, maniID)
	if err != nil {
		return mani, err
	}
	if !maniFound {
		return mani, fmt.Errorf("did not find manifest with ID %d", maniID)
	}
	return mani, nil
}

func (n Neo) getManiData(ctx context.Context, br utils.BootstrapResult, label string) (maniData types.ManifestData, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameInternal,
		"func":    "getMani",
	})
	defer span.End()
	_ = l
	maniDataID, e := br.GetInt64(fmt.Sprintf("%s#id", label))
	if e != nil {
		return maniData, e
	}
	maniData, maniDataFound, err := n.ManifestData_Get_ByID(ctx, maniDataID)
	if err != nil {
		return maniData, err
	}
	if !maniDataFound {
		return maniData, fmt.Errorf("did not find manifest with ID %d", maniDataID)
	}
	return maniData, nil
}

func (n Neo) getProfile(ctx context.Context, br utils.BootstrapResult, label string) (prf types.Profile, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameInternal,
		"func":    "getProfile",
	})
	defer span.End()
	_ = l
	prf, prfFound, err := n.Profile_Get_ByLabel(ctx, label)
	if err != nil {
		return prf, err
	}
	if !prfFound {
		return prf, fmt.Errorf("did not find profile with label: %s", label)
	}
	return
}
