package magazine

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func (mag Magazine) ComputeManifestlists(ctx context.Context) (err error) {
	logrus.Warnf("Compute Manifest Lists")
	err = mag.Database.Rule_EvalAll(ctx)
	if err != nil {
		return
	}
	/// TODO: We need
	/// 1. Generate ExprRes? (a rule<>mdID mapping)
	rls, err := mag.Database.Rules_GetAll(ctx)
	if err != nil {
		logrus.Errorf("failed to get rules: %v", err)
		return
	}
	er := types.NewExprRes()
	for _, rl := range rls {
		label := fmt.Sprintf("%s:%s", rl.Name, rl.Tag)
		logrus.Warnf("Rule: %s", label)
		mds, e := mag.Database.ManifestData_GetByRule(ctx, rl)
		if e != nil {
			err = errors.Wrapf(e, "failed to get manifestdata by rule: %v", err)
			return
		}
		for _, md := range mds {
			logrus.Warnf("  manifestdata: %s", md.Label)
			er.Add(label, md)
		}
	}
	/// 2. Query for profiles (set of rules) and output the manifestdata that evals true for all rules of a profile
	prfs, err := mag.Database.GetProfiles()
	if err != nil {
		logrus.Errorf("failed to get profiles: %v", err)
		return
	}
	for _, prf := range prfs {
		label := fmt.Sprintf("%s:%s", prf.Name, prf.Tag)
		if prf.Name == "" {
			label = prf.Tag
		}
		logrus.Warnf("Profile: %s", label)
		if prf.Rule != "" {
			lbs, err := utils.ExprGetLabels(prf.Rule)
			if err != nil {
				logrus.Errorf("failed to get labels from rule: %v", err)
				return err
			}
			mds := er.GetManifestdataForRules(lbs.ToSlice()...)
			for _, md := range mds {
				logrus.Warnf("  manifestdata: %s", md.Label)
			}
		}
	}
	ctpps, err := mag.Database.CollectionTagProfilePair_GetAll(ctx)
	if err != nil {
		logrus.Errorf("failed to get profile manifestdata pairs: %v", err)
		return
	}
	for _, ctpp := range ctpps {
		c := ctpp.Collection
		ct := ctpp.Collection.ColTag
		p := ctpp.Profile
		logrus.Infof("Collection: [%d]%s:%s[%d] [%s:%s]", c.ID, c.Repo, ct.Tag, ct.ID, ctpp.Profile.Name, ctpp.Profile.Tag)
		ml := types.ManifestList{
			SchemaVersion: 2,
			MediaType:     "application/vnd.oci.image.index.v1+json",
			Manifests:     []types.ManifestData{},
		}
		tmpMDS := er.FilterByCTPP(ctpp)
		ms := []types.ManifestData{}
		for _, m := range tmpMDS {
			// TODO: Kick out Manifests with a single FALSE relationship
			ok, err := mag.Database.ManifestData_CheckProfile(ctx, m, p)
			if err != nil {
				logrus.Errorf("Failed to check profile: %v", err)
				return err
			}
			if ok {
				ms = append(ms, m)
				ml.Manifests = append(ml.Manifests, m)
			}
		}
		if len(ms) == 0 {
			logrus.Warnf("  No Manifests for profile: %s", p.Name)
			continue
		}
		mlBlob := ml.ToOCIBytes()
		mlDigest := digest.FromBytes(mlBlob)
		err = mag.Minio.ManifestsPutObj(ctx, mlDigest.String(), mlBlob)
		if err != nil {
			logrus.Error(err)
			continue
		}
		mlID, err := mag.Database.InsertManifestList(mlDigest, ml.MediaType, len(mlBlob))
		if err != nil {
			logrus.Error(err)
			continue
		}
		// Link ManifestList to ManifestData
		for _, m := range ms {
			e := mag.Database.LinkManifestListToManifestData(mlID, m.ID)
			if e != nil {
				err = errors.Wrap(e, "while linking ManifestList to ManifestData")
				return err
			}
		}

		// Link ManifestList to Collection
		err = mag.Database.LinkManifestListToCollection(mlID, c.ID)
		if err != nil {
			logrus.Error(err)
			continue
		}
		// Link ManifestList to CollectionTag
		err = mag.Database.LinkManifestListToCollectionTag(mlID, ct.ID)
		if err != nil {
			logrus.Error(err)
			continue
		}
		// Link ManifestList to Profile
		err = mag.Database.LinkManifestListToProfile(mlID, p.ID)
		if err != nil {
			logrus.Error(err)
			continue
		}
	}
	return
}

func (mag Magazine) ManifestListGetByRequest(ctx context.Context, req types.Request) (ml types.ManifestList, found bool, err error) {
	ctx, span := tracer.Start(ctx, "ManifestListGetByRequest")
	defer span.End()
	// Fetch ML from SQLite
	ml, found, err = mag.Database.ManifestList_Get_ByRequest(ctx, req)
	if err != nil {
		logrus.Errorf("Failed to get ManifestList from Database: %v", err)
		return
	}
	if !found {
		// We did not find a ManifestList
		return
	}
	return
}

func (mag Magazine) ManifestList_GetByHTTPRequest(ctx context.Context, r *http.Request) (ml types.ManifestList, found bool, err error) {
	ctx, span := tracer.Start(ctx, "ManifestList_GetByHTTPRequest")
	defer span.End()
	req := types.NewRequest(ctx, r)
	msg := fmt.Sprintf("%-5s ManifestListHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	if req.Profile != "generic" {
		msg += fmt.Sprintf(" - profile: %s", req.Profile)
	}
	logrus.Info(msg)
	ml, found, err = mag.ManifestListGetByRequest(ctx, req)
	if err != nil {
		return
	}
	return
}

/*
func (mag Magazine) AddMLtoCollection(col *types.Collection) (err error) {
	q := fmt.Sprintf(`SELECT
			colID,
			colRepo,
			colTag,
			manifestlistDigest,
			manifestlistID
	FROM main.v_get_manifestlist
	WHERE colID=%d;`, col.SqlID)
	rows, err := mag.SQlite.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	type rType struct {
		ColID          int64
		ColRepo        string
		ColTag         string
		ManiListDigest string
		ManiListID     int64
	}
	for rows.Next() {
		mlRes := rType{}
		err = rows.Scan(
			&mlRes.ColID,
			&mlRes.ColRepo,
			&mlRes.ColTag,
			&mlRes.ManiListDigest,
			&mlRes.ManiListID,
		)
		if err != nil {
			logrus.Error(err)
		}

		ml := types.ManifestList{
			SchemaVersion: 2,
			MediaType:     "application/vnd.docker.distribution.manifest.list.v2+json",
			SqlID:         int64(mlRes.ManiListID),
			Digest:        mlRes.ManiListDigest,
			Manifests:     []types.Manifest{},
		}
		/// Add manifests to manifestlist
		manis, err := mag.SQlite.Manifest.TypesManifest_GetByManifestListID(ml.SqlID)
		if err != nil {
			logrus.Error(err)
			continue
		}
		ml.Manifests = manis
		col.ManifestLists = append(col.ManifestLists, ml)
	}
	return
}
*/
