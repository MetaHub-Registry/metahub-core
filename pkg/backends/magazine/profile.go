package magazine

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (mag Magazine) NeoInsertProfile(ctx context.Context, prf types.Profile) (prfID int64, err error) {
	prfID, success, err := mag.Neo.GetProfile(prf)
	if err != nil {
		return
	}
	if !success {
		prfID, err = mag.Neo.CreateProfile(prf)
		if err != nil {
			return
		}
	}
	return
}

func (mag Magazine) SQliteLinkProfileToFilter(ctx context.Context, prfID, filterID int64) (err error) {
	return mag.SQlite.LinkProfileToFilter(ctx, prfID, filterID)
}

func (mag Magazine) NeoLinkProfileToFilter(ctx context.Context, prfID, filterID int64) (err error) {
	return mag.Neo.LinkProfileToFilter(ctx, prfID, filterID)
}

func (mag Magazine) SQliteInsertProfile(ctx context.Context, p types.Profile) (profileID int64, err error) {
	profileID, success, err := mag.SQlite.Profile.HasProfile(p.Name)
	if err != nil {
		return
	}
	if !success {
		err = mag.SQlite.Profile.Insert(&p)
		if err != nil {
			return
		}
		profileID = p.SqlID
	}
	return
}
