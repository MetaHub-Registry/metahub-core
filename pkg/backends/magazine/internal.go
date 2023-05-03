package magazine

import (
	"context"
	"fmt"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func (mag Magazine) getMani(ctx context.Context, br utils.BootstrapResult, label string) (mani types.Manifest, err error) {
	maniID, e := br.GetInt64(fmt.Sprintf("%s#id", label))
	if e != nil {
		return mani, e
	}
	mani, maniFound, err := mag.Database.Manifest_GetByID(ctx, maniID)
	if err != nil {
		return mani, err
	}
	if !maniFound {
		return mani, fmt.Errorf("did not find manifest with ID %d", maniID)
	}
	return mani, nil
}

func (mag Magazine) getProfile(ctx context.Context, br utils.BootstrapResult, label string) (prf types.Profile, err error) {
	prf, prfFound, err := mag.Database.Profile_Get_ByLabel(ctx, label)
	if err != nil {
		return prf, err
	}
	if !prfFound {
		return prf, fmt.Errorf("did not find profile with label: %s", label)
	}
	return
}

func (mag Magazine) getManiData(ctx context.Context, br utils.BootstrapResult, label string) (mani types.ManifestData, err error) {
	mdID, e := br.GetInt64(fmt.Sprintf("%s#id", label))
	if e != nil {
		return mani, e
	}
	mani, maniFound, err := mag.ManifestData_Get_ByID(ctx, mdID)
	if err != nil {
		return mani, err
	}
	if !maniFound {
		return mani, fmt.Errorf("did not find ManifestData with ID %d", mdID)
	}
	return mani, nil
}
