package magazine

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
	"go.opentelemetry.io/otel/attribute"
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
	colID, colTagID, success, err := mag.Database.Collection_GetID_ByYaml(ctx, col)
	if err != nil {
		l.Error(err.Error())
		return
	}
	if !success {
		colID, colTagID, err = mag.Database.Collection_Create_ByYaml(ctx, col)
		if err != nil {
			l.Error(err.Error())
			return
		}
	}
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

func (mag Magazine) HeadManifestNotFound(ctx context.Context, req types.Request) (found bool, err error) {
	ctx, span := tracer.Start(ctx, "HeadManifestNotFound")
	defer span.End()
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
	col := yamlCfg.NewManifestCollection(req.Repo, req.Tag)
	colID, colTagID, e := mag.CreateCollection(ctx, col)
	if e != nil {
		err = errors.Wrap(e, "failed to create collection")
		l.Error(err.Error())
		span.RecordError(err)
		return
	}
	msg = fmt.Sprintf("collection '%s'[%d] and tag '%s'[%d] created", req.Repo, colID, req.Tag, colTagID)
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
		msg = fmt.Sprintf("upstream '%s:%s' found in registry '%s'", req.Repo, req.Tag, reg)
		l.Warn(msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
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
