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


func (n Neo) getManiData(ctx context.Context, br utils.BootstrapResult, label string) (maniData types.Manifest, err error) {
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
	maniData, maniDataFound, err := n.ManifestGetByID(ctx, maniDataID)
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
	_ = br
	prf, prfFound, err := n.ProfileGetByLabel(ctx, label)
	if err != nil {
		return prf, err
	}
	if !prfFound {
		return prf, fmt.Errorf("did not find profile with label: %s", label)
	}
	return
}
