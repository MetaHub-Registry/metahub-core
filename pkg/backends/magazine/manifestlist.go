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

func (mag Magazine) ComputeManifestlist_ForCTTP(ctx context.Context, er *types.ExprRes, ctpp types.CollectionTagProfilePair) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "ComputeManifestlist_ForCTTP",
		"cttp":    ctpp.String(),
	})
	defer span.End()
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
	tmpMDS := er.FilterByCTPP(ctpp)
	ms := []types.ManifestData{}
	for _, m := range tmpMDS {
		// TODO: Kick out Manifests with a single FALSE relationship
		ok, e := mag.Database.ManifestDataCheckProfile(ctx, m, p)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "Failed to check profile")
		}
		if ok {
			utils.Debug(l, span, "Adding Manifest to ManifestList: %s", m.Upstream.String())
			ms = append(ms, m)
			ml.Manifests = append(ml.Manifests, m)
		}
	}
	if len(ms) == 0 {
		utils.Warn(l, span, "No Manifests for profile: %s", p.Name)
		return
	}
	mlBlob := ml.ToOCIBytes()
	mlDigest := digest.FromBytes(mlBlob)
	e := mag.Minio.ManifestsPutObj(ctx, mlDigest.String(), mlBlob)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed ManifestsPutObj")
	}
	mlID, e := mag.Database.InsertManifestList(mlDigest, ml.MediaType, len(mlBlob))
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed InsertManifestList")
	}
	// Link ManifestList to ManifestData
	for _, m := range ms {
		e := mag.Database.LinkManifestListToManifestData(mlID, m.ID)
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
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/manifestlist.go",
		"func":    "ComputeManifestlists",
	})
	ctx, span := tracer.Start(ctx, "ComputeManifestlists")
	defer span.End()
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
	rls, err := mag.Database.Rules_GetAll(ctx)
	if err != nil {
		err = errors.Wrap(err, "failed to get rules")
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	er := types.NewExprRes()
	for _, rl := range rls {
		label := fmt.Sprintf("%s:%s", rl.Name, rl.Tag)
		msg := fmt.Sprintf("Rule: '%s'", label)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Debug(msg)
		mds, e := mag.Database.ManifestData_GetByRule(ctx, rl)
		if e != nil {
			err = errors.Wrapf(e, "failed to get manifestdata by rule: %v", err)
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
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
	utils.Info(l, span, "Computing manifestlists for regexes: %s, %s, %s", repoRegex, tagRegex, prfRegex)
	ctpps, err := mag.Database.CollectionTagProfilePairGetRegexFiltered(ctx, repoRegex, tagRegex, prfRegex)
	if err != nil {
		err = errors.Wrap(err, "failed to get profile manifestdata pairs")
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	for _, ctpp := range ctpps {
		err = mag.ComputeManifestlist_ForCTTP(ctx, er, ctpp)
		if err != nil {
			err = errors.Wrapf(err, "failed to compute manifestlist for cttp: %v", ctpp)
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			continue
		}
	}
	return
}

// ComputeManifestlists_ForManiProfile computes manifestlists for a given Manifest and Profile
func (mag Magazine) ComputeManifestlists_ForManiProfile(ctx context.Context, maniID, prfID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "magazine/manifestlist.go",
		"func":    "ComputeManifestlists_ForManiProfile",
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
