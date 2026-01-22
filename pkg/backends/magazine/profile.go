package magazine

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (mag Magazine) NeoInsertProfile(ctx context.Context, prf types.Profile) (prfID int64, err error) {
	/*prfID, success, err := mag.Neo.GetProfile(prf)
	if err != nil {
		return
	}
	if !success {
		prfID, err = mag.Neo.CreateProfile(prf)
		if err != nil {
			return
		}
	}
	*/
	err = mag.Database.ProfileGetInsert(ctx, &prf)
	if err != nil {
		logrus.Errorf("Failed to insert profile: %v", err)
		return
	}
	prfID = prf.ID
	return
}

/*func (mag Magazine) SQliteLinkProfileToFilter(ctx context.Context, prfID, filterID int64) (err error) {
	return mag.Database.LinkProfileToFilter(ctx, prfID, filterID)
}

func (mag Magazine) NeoLinkProfileToFilter(ctx context.Context, prfID, filterID int64) (err error) {
	return mag.Database.LinkProfileToFilter(ctx, prfID, filterID)
}
*/

func (mag Magazine) SQliteInsertProfile(ctx context.Context, p types.Profile) (profileID int64, err error) {
	err = mag.Database.ProfileGetInsert(ctx, &p)
	if err != nil {
		return
	}
	return p.ID, nil
}
