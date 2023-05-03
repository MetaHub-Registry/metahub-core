package utils

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/helper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func ErrorHandler(l *logrus.Entry, span trace.Span, e error, frmt string, a ...any) (err error) {
	msg := fmt.Sprintf(frmt, a...)
	err = errors.Wrap(e, msg)
	span.SetStatus(codes.Error, err.Error())
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "error"),
		attribute.String("log.message", err.Error()),
	))
	span.RecordError(err)
	l.Error(err.Error())
	return
}

func ErrorHandler_WithWWW(w http.ResponseWriter, l *logrus.Entry, span trace.Span, repo, action string, e error, eType, frmt string, a ...any) (err error) {
	var msg string
	msg = e.Error()
	err = e
	if frmt != "" {
		msg = fmt.Sprintf(frmt, a...)
		err = errors.Wrap(e, fmt.Sprintf("%s - %s", eType, msg))
	}
	span.SetStatus(codes.Error, err.Error())
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "error"),
		attribute.String("log.message", err.Error()),
	))
	span.RecordError(err)
	l.Error(err.Error())
	det := helper.NewDetail("repositry", repo, action)
	ep := helper.NewErrPayload(eType, msg, det)
	switch eType {
	case "MANIFEST_UNKNOWN":
		helper.WriteOCIErrror(w, http.StatusNotFound, ep)
	default:
		helper.WriteOCIErrror(w, http.StatusInternalServerError, ep)
	}

	return
}

func NewError(l *logrus.Entry, span trace.Span, frmt string, a ...any) (err error) {
	err = fmt.Errorf(frmt, a...)
	span.SetStatus(codes.Error, err.Error())
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "error"),
		attribute.String("log.message", err.Error()),
	))
	span.RecordError(err)
	l.Error(err.Error())
	return
}

func Info(l *logrus.Entry, span trace.Span, frmt string, a ...any) {
	msg := fmt.Sprintf(frmt, a...)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "info"),
		attribute.String("log.message", msg),
	))
	l.Info(msg)
}

func Debug(l *logrus.Entry, span trace.Span, frmt string, a ...any) {
	msg := fmt.Sprintf(frmt, a...)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Debug(msg)
}
func DebugOK(l *logrus.Entry, span trace.Span, frmt string, a ...any) {
	msg := fmt.Sprintf(frmt, a...)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	span.SetStatus(codes.Ok, msg)
	l.Debug(msg)
}

func Warn(l *logrus.Entry, span trace.Span, frmt string, a ...any) {
	msg := fmt.Sprintf(frmt, a...)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "warn"),
		attribute.String("log.message", msg),
	))
	l.Warn(msg)
}

func InitLogging(ctx context.Context, tracer trace.Tracer, attr map[string]any) (l *logrus.Entry, c context.Context, span trace.Span) {
	pkg, ok := attr["package"].(string)
	if !ok {
		pkg = "FILLME"
	} else {
		delete(attr, "package")
	}
	fnc, ok := attr["func"].(string)
	if !ok {
		pkg = "FILLME"
	} else {
		delete(attr, "func")
	}
	c, span = tracer.Start(ctx, fnc)

	l = logrus.WithFields(logrus.Fields{
		"package": pkg,
		"func":    fnc,
	})
	for k, v := range attr {
		if k == "digest" {
			v = ShortenDigest(v.(string))
		}
		l = l.WithField(k, v)
		switch v.(type) {
		case int, int64:
			span.SetAttributes(attribute.Key(k).String(fmt.Sprintf("%d", v)))
		case string:
			span.SetAttributes(attribute.Key(k).String(v.(string)))
		default:
			NewError(l, span, "Could not set attribute %s : %v", k, v)
		}

	}
	return
}
