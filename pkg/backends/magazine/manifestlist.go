package magazine

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (mag Magazine) ComputeManifestlist_ForCTTP(ctx context.Context, er *types.ExprRes, ctpp types.CollectionTagProfilePair) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/manifestlist.go",
		"func":    "ComputeManifestlist_ForCTTP",
	})
	ctx, span := tracer.Start(ctx, "ComputeManifestlists")
	defer span.End()
	l.Infof("Compute Manifest Lists for CTTP: %s", ctpp.String())
	c := ctpp.Collection
	ct := ctpp.Collection.ColTag
	p := ctpp.Profile
	l.Infof("Collection: [%d]%s:%s[%d] [%s:%s]", c.ID, c.Repo, ct.Tag, ct.ID, ctpp.Profile.Name, ctpp.Profile.Tag)
	ml := types.ManifestList{
		SchemaVersion: 2,
		MediaType:     v1.MediaTypeImageManifest,
		Manifests:     []types.ManifestData{},
	}
	tmpMDS := er.FilterByCTPP(ctpp)
	ms := []types.ManifestData{}
	for _, m := range tmpMDS {
		// TODO: Kick out Manifests with a single FALSE relationship
		ok, err := mag.Database.ManifestData_CheckProfile(ctx, m, p)
		if err != nil {
			l.Errorf("Failed to check profile: %v", err)
			return err
		}
		if ok {
			ms = append(ms, m)
			ml.Manifests = append(ml.Manifests, m)
		}
	}
	if len(ms) == 0 {
		l.Warnf("  No Manifests for profile: %s", p.Name)
		return
	}
	mlBlob := ml.ToOCIBytes()
	mlDigest := digest.FromBytes(mlBlob)
	err = mag.Minio.ManifestsPutObj(ctx, mlDigest.String(), mlBlob)
	if err != nil {
		l.Error(err)
		return
	}
	mlID, err := mag.Database.InsertManifestList(mlDigest, ml.MediaType, len(mlBlob))
	if err != nil {
		l.Error(err)
		return
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
		l.Error(err)
		return
	}
	// Link ManifestList to CollectionTag
	err = mag.Database.LinkManifestListToCollectionTag(mlID, ct.ID)
	if err != nil {
		l.Error(err)
		return
	}
	// Link ManifestList to Profile
	err = mag.Database.LinkManifestListToProfile(mlID, p.ID)
	if err != nil {
		l.Error(err)
		return
	}
	return
}

func (mag Magazine) ComputeManifestlists(ctx context.Context) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/manifestlist.go",
		"func":    "ComputeManifestlists",
	})
	ctx, span := tracer.Start(ctx, "ComputeManifestlists")
	defer span.End()
	l.Info("Compute Manifest Lists")
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
		l.Warnf("Rule: %s", label)
		mds, e := mag.Database.ManifestData_GetByRule(ctx, rl)
		if e != nil {
			err = errors.Wrapf(e, "failed to get manifestdata by rule: %v", err)
			return
		}
		for _, md := range mds {
			//l.Warnf("  manifestdata: %s", md.Label)
			er.Add(label, md)
		}
	}
	repoRegex, ok := mag.Config.Args["repoRegex"]
	if !ok || repoRegex == "" {
		repoRegex = ".*"
	}
	tagRegex, ok := mag.Config.Args["tagRegex"]
	if !ok || tagRegex == "" {
		tagRegex = ".*"
	}
	prfRegex, ok := mag.Config.Args["profileRegex"]
	if !ok || prfRegex == "" {
		prfRegex = ".*"
	}
	ctpps, err := mag.Database.CollectionTagProfilePair_GetRegexFiltered(ctx, repoRegex, tagRegex, prfRegex)
	//ctpps, err := mag.Database.CollectionTagProfilePair_GetAll(ctx)
	if err != nil {
		l.Errorf("failed to get profile manifestdata pairs: %v", err)
		return
	}
	for _, ctpp := range ctpps {
		err = mag.ComputeManifestlist_ForCTTP(ctx, er, ctpp)
		if err != nil {
			err = errors.Wrapf(err, "failed to compute manifestlist for cttp: %v", ctpp)
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
