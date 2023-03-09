package types

import (
	"testing"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
)

var (
	tracer = otel.Tracer("metalib/types")
)

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	m.Run()
}
