package magazine

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (mag Magazine) CreateCollection(ctx context.Context, col yamlCfg.ManifestCollection) (colID, colTagID int64, err error) {
	_, span := tracer.Start(ctx, "CreateCollection")
	defer span.End()
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/collection.go",
		"func":    "CreateCollection",
		"repo":    col.Name,
		"tag":     col.Tag,
	})
	colID, colTagID, success, e := mag.Database.Collection_GetID_ByYaml(ctx, col)
	if e != nil {
		err = errors.Wrap(e, "failed to Collection_GetID_ByYaml")
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	if !success {
		colID, colTagID, e = mag.Database.Collection_Create_ByYaml(ctx, col)
		if e != nil {
			err = errors.Wrap(e, "failed to Collection_Create_ByYaml")
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return
		}
	}
	msg := fmt.Sprintf("colID:%d / colTagID:%d", colID, colTagID)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Debug(msg)
	return
}

func (mag Magazine) NeoLinkCollectionToManifest(ctx context.Context, colID, colTagID, maniID int64) (err error) {
	err = mag.Database.LinkCollectionToManifest(ctx, colID, maniID)
	if err != nil {
		return
	}
	err = mag.Database.LinkCollectionTagToManifest(ctx, colTagID, maniID)
	return
}

func (mag Magazine) RemoveCollectionByRequest(req types.Request) (found bool, err error) {
	ctx := context.Background()
	c := types.Collection{
		Repo: req.Repo,
		Tag:  req.Tag,
	}
	return mag.Database.Collection_Remove(ctx, c)
}

func (mag Magazine) HeadManifestNotFound(ctx context.Context, req types.Request) (md string, found bool, err error) {
	ctx, span := tracer.Start(ctx, "HeadManifestNotFound")
	defer span.End()
	span.SetAttributes(
		attribute.Key("repo").String(req.Repo),
		attribute.Key("tag").String(req.Tag),
	)
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/collection.go",
		"func":    "CreateCollection",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
	})
	msg := fmt.Sprintf("collection '%s:%s' not found", req.Repo, req.Tag)
	l.Warn(msg)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	// DONE: Create Collection
	col, _, e := mag.Database.Collection_GetCreate_ByRepoTag(ctx, req.Repo, req.Tag)
	if e != nil {
		err = errors.Wrap(e, "while Collection_GetCreate_ByRepoTag")
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	msg = fmt.Sprintf("collection [%d] and tag [%d] created", col.ID, col.ColTag.ID)
	l.Warn(msg)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	// DONE: Discover Upstream image
	blob, reg, found, e := mag.DiscoverUpstream(ctx, req.Repo, req.Tag)
	if e != nil {
		err = errors.Wrap(e, "failed to discover upstream")
		l.Error(err.Error())
		span.RecordError(err)
		return
	}
	req.Host = reg
	if found {
		msg = fmt.Sprintf("upstream found in registry '%s'. MediaType: %s", reg, blob.MediaType)
		l.Warn(msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		md = blob.MediaType
		switch blob.MediaType {
		case "application/vnd.docker.distribution.manifest.v2+json":
			// Insert into DB
			e := mag.UpstreamBlob_InsertManifest(ctx, req, blob)
			if e != nil {
				err = errors.Wrap(e, "failed to insert upstreamblob collection")
				l.Error(err.Error())
				span.RecordError(err)
				return
			}

		case "application/vnd.docker.distribution.manifest.list.v2+json":
			e := mag.UpstreamBlob_InsertIndex(ctx, req, blob)
			if e != nil {
				err = errors.Wrap(e, "failed to insert upstreamblob collection")
				l.Error(err.Error())
				span.RecordError(err)
				return
			}

		default:
			err = fmt.Errorf("unsuported media type: %s", blob.MediaType)
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return
		}
	} else {
		msg = fmt.Sprintf("upstream '%s:%s' not found'", req.Repo, req.Tag)
		l.Warn(msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		found = false
	}
	return
}
