package utils

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func ErrorHandler(l *logrus.Entry, span trace.Span, e error, msg string) {
	err := errors.Wrap(e, msg)
	span.SetStatus(codes.Error, err.Error())
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "error"),
		attribute.String("log.message", err.Error()),
	))
	span.RecordError(err)
	l.Error(err.Error())
}

func NewError(l *logrus.Entry, span trace.Span, msg string) (err error) {
	err = errors.New(msg)
	span.SetStatus(codes.Error, err.Error())
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "error"),
		attribute.String("log.message", err.Error()),
	))
	span.RecordError(err)
	l.Error(err.Error())
	return
}

func Debug(l *logrus.Entry, span trace.Span, frmt string, a ...any) {
	msg := fmt.Sprintf(frmt, a...)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Debug(msg)
}
