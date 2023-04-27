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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (mag Magazine) ComputeManifestlist_ForCTTP(ctx context.Context, er *types.ExprRes, ctpp types.CollectionTagProfilePair) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/manifestlist.go",
		"func":    "ComputeManifestlist_ForCTTP",
		"cttp":    ctpp.String(),
	})
	ctx, span := tracer.Start(ctx, "ComputeManifestlist_ForCTTP")
	defer span.End()
	span.SetAttributes(attribute.String("cttp", ctpp.String()))
	l.Infof("Compute Manifest Lists for CTTP: %s", ctpp.String())
	c := ctpp.Collection
	ct := ctpp.Collection.ColTag
	p := ctpp.Profile
	msg := fmt.Sprintf("Collection: '%s'", ctpp.String())
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "info"),
		attribute.String("log.message", msg),
	))
	l.Info(msg)
	ml := types.ManifestList{
		SchemaVersion: 2,
		MediaType:     v1.MediaTypeImageIndex,
		Manifests:     []types.ManifestData{},
	}
	tmpMDS := er.FilterByCTPP(ctpp)
	ms := []types.ManifestData{}
	for _, m := range tmpMDS {
		// TODO: Kick out Manifests with a single FALSE relationship
		ok, e := mag.Database.ManifestData_CheckProfile(ctx, m, p)
		if e != nil {
			err = errors.Wrap(e, "Failed to check profile")
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return err
		}
		if ok {
			msg := fmt.Sprintf("Adding Manifest to ManifestList: %s", m.Upstream.String())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", msg),
			))
			l.Debug(msg)
			ms = append(ms, m)
			ml.Manifests = append(ml.Manifests, m)
		}
	}
	if len(ms) == 0 {
		msg := fmt.Sprintf("No Manifests for profile: %s", p.Name)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "warn"),
			attribute.String("log.message", msg),
		))
		l.Warn(msg)
		return
	}
	mlBlob := ml.ToOCIBytes()
	mlDigest := digest.FromBytes(mlBlob)
	e := mag.Minio.ManifestsPutObj(ctx, mlDigest.String(), mlBlob)
	if e != nil {
		err = errors.Wrap(e, "failed ManifestsPutObj")
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	mlID, e := mag.Database.InsertManifestList(mlDigest, ml.MediaType, len(mlBlob))
	if e != nil {
		err = errors.Wrap(e, "failed InsertManifestList")
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	// Link ManifestList to ManifestData
	for _, m := range ms {
		e := mag.Database.LinkManifestListToManifestData(mlID, m.ID)
		if e != nil {
			err = errors.Wrap(e, "while linking ManifestList to ManifestData")
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return err
		}
	}

	// Link ManifestList to Collection
	e = mag.Database.LinkManifestListToCollection(mlID, c.ID)
	if e != nil {
		err = errors.Wrap(e, "failed LinkManifestListToCollection")
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	// Link ManifestList to CollectionTag
	e = mag.Database.LinkManifestListToCollectionTag(mlID, ct.ID)
	if e != nil {
		err = errors.Wrap(e, "failed LinkManifestListToCollectionTag")
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	// Link ManifestList to Profile
	e = mag.Database.LinkManifestListToProfile(mlID, p.ID)
	if e != nil {
		err = errors.Wrap(e, "failed LinkManifestListToCollectionTag")
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
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
	err = mag.Database.Rule_EvalAll(ctx)
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
	ctpps, err := mag.Database.CollectionTagProfilePair_GetRegexFiltered(ctx, repoRegex, tagRegex, prfRegex)
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
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/manifestlist.go",
		"func":    "ComputeManifestlists_ForManiProfile",
	})
	ctx, span := tracer.Start(ctx, "ComputeManifestlists_ForManiProfile")
	defer span.End()
	l.Info("Compute Manifest Lists")
	err = mag.Database.Rule_EvalAll(ctx)
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
	ctpps, err := mag.Database.CollectionTagProfilePair_GetRegexFiltered(ctx, repoRegex, tagRegex, prfRegex)
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
