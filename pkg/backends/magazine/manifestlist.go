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
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameML = "magazine/manifestlist.go"
)

func (mag Magazine) ComputeManifestlistForCTTP(ctx context.Context, er *types.ExprRes, ctpp types.CollectionTagProfilePair) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "ComputeManifestlistForCTTP",
		"cttp":    ctpp.String(),
	})
	defer span.End()
	if er.IsEmpty() {
		return utils.NewError(l, span, "ExprRes is empty, skipping manifestlist computation for CTTP: %s", ctpp.String())
	}
	span.SetAttributes(attribute.String("cttp", ctpp.String()))
	utils.Info(l, span, "Compute Manifest Lists for CTTP: %s", ctpp.String())
	c := ctpp.Collection
	ct := ctpp.Collection.ColTag
	p := ctpp.Profile
	utils.Info(l, span, "Collection: '%s'", ctpp.String())
	ml := types.ManifestList{
		SchemaVersion: 2,
		MediaType:     v1.MediaTypeImageIndex,
		Manifests:     []types.ManifestData{},
	}
	tmpMDS := er.FilterByCTPP(ctx, ctpp)
	utils.Debug(l, span, "MDS: %d", len(tmpMDS))
	mds := []types.ManifestData{}
	for _, m := range tmpMDS {
		utils.Debug(l, span, "ManifestDataID:%d, Platform:%s", m.ID, m.Platform.GetLabel())
		// TODO: Kick out Manifests with a single FALSE relationship
		ok, e := mag.Database.ManifestDataCheckProfile(ctx, m, p)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "Failed to check profile")
		}
		if ok {
			utils.Debug(l, span, "Adding Manifest to ManifestList: %s", m.Upstream.String())
			mds = append(mds, m)
			ml.Manifests = append(ml.Manifests, m)
		}
	}
	if len(mds) == 0 {
		utils.Warn(l, span, "No Manifests for profile: %s", p.Name)
		return
	}
	utils.Info(l, span, "Storing ManifestList with %d Manifests for CTTP: %s", len(mds), ctpp.String())
	mlBlob := ml.ToOCIBytes()
	mlDigest := digest.FromBytes(mlBlob)
	e := mag.Minio.ManifestsPutObj(ctx, mlDigest.String(), mlBlob)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed ManifestsPutObj")
	}
	mlID, e := mag.Database.InsertManifestList(ctx, mlDigest, ml.MediaType, len(mlBlob))
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed InsertManifestList")
	}
	// Link ManifestList to ManifestData
	for _, md := range mds {
		utils.Debug(l, span, " Linking ManifestData ID:%d to ManifestList", md.ID)
		if mdID, found, err := mag.Database.ManifestDataGetByDigest(ctx, md.Digest); err != nil {
			return utils.ErrorHandler(l, span, err, "failed ManifestDataGetByDigest")
		} else if !found {
			return utils.NewError(l, span, "ManifestData not found by digest: %s", md.Digest)
		} else {
			md.ID = mdID.ID
		}
		e := mag.Database.LinkManifestListToManifestData(ctx, mlID, md.ID)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "while linking ManifestList to ManifestData")
		}
	}
	// Link ManifestList to Collection
	e = mag.Database.LinkManifestListToCollection(mlID, c.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed LinkManifestListToCollection")
	}
	// Link ManifestList to CollectionTag
	e = mag.Database.LinkManifestListToCollectionTag(mlID, ct.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed LinkManifestListToCollectionTag")
	}
	// Link ManifestList to Profile
	e = mag.Database.LinkManifestListToProfile(mlID, p.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed LinkManifestListToCollectionTag")
	}
	return
}

func (mag Magazine) ComputeManifestlists(ctx context.Context) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "magazine/manifestlist.go",
		"func":    "ComputeManifestlists",
	})
	defer span.End()
	err = mag.Database.RuleEvalAll(ctx)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "while evaluating all rules")
		return
	}
	/// TODO: We need
	/// 1. Generate ExprRes? (a rule<>mdID mapping)
	rls, err := mag.Database.RulesGetAll(ctx)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get rules")
		return
	}
	utils.Debug(l, span, "Rules fetched: %d", len(rls))
	er := types.NewExprRes()
	for _, rl := range rls {
		label := fmt.Sprintf("%s:%s", rl.Name, rl.Tag)
		utils.Debug(l, span, "Processing Rule: %s", label)
		mds, e := mag.Database.ManifestDataGetByRule(ctx, rl)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get manifestdata by rule")
			return
		}
		for _, md := range mds {
			utils.Debug(l, span, "  manifestdata: %s", md.Label)
			er.Add(label, md)
		}
	}
	if er.IsEmpty() {
		utils.Warn(l, span, "No manifestdata found in ExprRes, skipping manifestlist computation")
		return
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
	utils.Info(l, span, "Computing manifestlists for regexes: %s, %s, %s", repoRegex, tagRegex, prfRegex)
	ctpps, err := mag.Database.CollectionTagProfilePairGetRegexFiltered(ctx, repoRegex, tagRegex, prfRegex)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get profile manifestdata pairs")
		return
	}
	for _, ctpp := range ctpps {
		err = mag.ComputeManifestlistForCTTP(ctx, er, ctpp)
		if err != nil {
			err = utils.ErrorHandler(l, span, err, "failed to compute manifestlist for cttp: %v", ctpp)
			continue
		}
	}
	return
}

// ComputeManifestlistsForManiProfile computes manifestlists for a given Manifest and Profile
func (mag Magazine) ComputeManifestlistsForManiProfile(ctx context.Context, maniID, prfID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "magazine/manifestlist.go",
		"func":    "ComputeManifestlistsForManiProfile",
	})
	defer span.End()
	l.Info("Compute Manifest Lists")
	err = mag.Database.RuleEvalAll(ctx)
	if err != nil {
		err = errors.Wrap(err, "while evaluating all rules")
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	/// TODO: We need
	/// 1. Generate ExprRes? (a rule<>mdID mapping)
	rls, err := mag.Database.RulesGetAll(ctx)
	if err != nil {
		logrus.Errorf("failed to get rules: %v", err)
		return
	}
	er := types.NewExprRes()
	for _, rl := range rls {
		label := fmt.Sprintf("%s:%s", rl.Name, rl.Tag)
		l.Warnf("Rule: %s", label)
		mds, e := mag.Database.ManifestDataGetByRule(ctx, rl)
		if e != nil {
			err = errors.Wrapf(e, "failed to get manifestdata by rule: %v", err)
			return
		}
		for _, md := range mds {
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
	ctpps, err := mag.Database.CollectionTagProfilePairGetRegexFiltered(ctx, repoRegex, tagRegex, prfRegex)
	if err != nil {
		l.Errorf("failed to get profile manifestdata pairs: %v", err)
		return
	}

	for _, ctpp := range ctpps {
		utils.Warn(l, span, "CTTP.ManifestData: %d", len(ctpp.Manifests))
		err = mag.ComputeManifestlistForCTTP(ctx, er, ctpp)
		if err != nil {
			err = errors.Wrapf(err, "failed to compute manifestlist for cttp: %v", ctpp)
			continue
		}
	}
	return
}

func (mag Magazine) ManifestListGetByRequest(ctx context.Context, req types.Request) (ml types.ManifestList, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "ManifestListGetByRequest",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  utils.ShortenDigest(req.Digest),
	})
	defer span.End()
	_ = l
	// Fetch ML from SQLite
	ml, found, err = mag.Database.ManifestListGetByRequest(ctx, req)
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

func (mag Magazine) ManifestListGetByHTTPRequest(ctx context.Context, r *http.Request) (ml types.ManifestList, found bool, err error) {
	ctx, span := tracer.Start(ctx, "ManifestListGetByHTTPRequest")
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

// func (mag Magazine) ManifestListPutByRequest(ctx context.Context, req types.Request, buf []byte) (ml types.ManifestList, err error) {
// 	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
// 		"package": pkgNameML,
// 		"func":    "ManifestListPutByRequest",
// 		"repo":    req.Repo,
// 		"tag":     req.Tag,
// 		"digest":  utils.ShortenDigest(req.Digest),
// 	})
// 	defer span.End()

//		err = mag.Database.ManifestListInsert(ctx, req, buf)
//		return
//	}
func (mag Magazine) StoreManifestListUpload(ctx context.Context, req types.Request, ml types.ManifestList) (mlId int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "StoreManifestListUpload",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  utils.ShortenDigest(req.Digest),
	})
	defer span.End()
	utils.Debug(l, span, "%s", ml.String())
	// Check if ManifestList already exists
	existingML, err := mag.Database.ManifestListGetByDigest(ctx, ml.Digest)
	if err != nil {
		return 0, utils.ErrorHandler(l, span, err, "while checking existing ManifestList by digest")
	}
	if existingML != nil {
		utils.Info(l, span, "ManifestList with Digest %s already exists, skipping insert", ml.Digest)
		return existingML.ID, nil
	}
	utils.Debug(l, span, "No ManifestList found")
	// Store ManifestList Blob in Minio
	mlBlob := ml.ToOCIBytes()
	mlDigest := digest.FromBytes(mlBlob)
	err = mag.Minio.ManifestsPutObj(ctx, mlDigest.String(), mlBlob)
	if err != nil {
		return 0, utils.ErrorHandler(l, span, err, "failed ManifestsPutObj")
	}
	// Insert ManifestList in Database
	mlId, err = mag.Database.InsertManifestList(ctx, mlDigest, ml.MediaType, len(mlBlob))
	if err != nil {
		return 0, utils.ErrorHandler(l, span, err, "failed InsertManifestList")
	}
	// Link ManifestList to ManifestData
	for _, md := range ml.Manifests {
		utils.Debug(l, span, " ManifestList contains ManifestData: %s", md.Platform.Arch)
		utils.Debug(l, span, " Linking ManifestData ID:%d to ManifestList", md.ID)
		if mdID, found, err := mag.Database.ManifestDataGetByDigest(ctx, md.Digest); err != nil {
			return mlId, utils.ErrorHandler(l, span, err, "failed ManifestDataGetByDigest")
		} else if !found {
			return mlId, utils.NewError(l, span, "ManifestData not found by digest: %s", md.Digest)
		} else {
			md.ID = mdID.ID
		}
		err = mag.Database.LinkManifestListToManifestData(ctx, mlId, md.ID)
		if err != nil {
			return 0, utils.ErrorHandler(l, span, err, "while linking ManifestList to ManifestData")
		}
	}
	return mlId, nil
}
