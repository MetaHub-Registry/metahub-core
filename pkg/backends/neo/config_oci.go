package neo

import (
	"context"
	"fmt"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (n Neo) OCIv1ConfigGetInsertDescriptor(ctx context.Context, desc v1.Descriptor) (cfg types.Config, found bool, err error) {
	shortDig := utils.ShortenDigest(desc.Digest.String())
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/config_oci.go",
		"func":    "OCIv1ConfigGetInsertDescriptor",
		"digest":  shortDig,
	})
	ctx, span := tracer.Start(ctx, "OCIv1ConfigGetInsertDescriptor")
	defer span.End()
	span.SetAttributes(
		attribute.Key("digest").String(shortDig),
	)
	cfg, found, err = n.ConfigGetByDigest(ctx, desc.Digest.String())
	if err != nil {
		return
	}
	if found {
		msg := fmt.Sprintf("Config with digest '%s' already exists", desc.Digest.String())
		logrus.Warn(msg)
		return
	}
	label := utils.CreateLabelFromDescriptor(desc)
	q := fmt.Sprintf(`CREATE (c:Blob) SET c.digest = "%s", c.size = %d, c.label ="%s", c.type = "config", c.mediaType = "%s"`, desc.Digest.String(), desc.Size, label, desc.MediaType)
	q += ` RETURN id(c) as configID, c.size AS configSize, c.digest AS configDigest`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, err := n.Execute(q)
	if err != nil {
		return
	}
	cfg, e := n.ConfigFromResult(ctx, res)
	if e != nil {
		err = errors.Wrap(e, "failed to create Config from result")
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	cfg.MediaType = desc.MediaType
	return
}
