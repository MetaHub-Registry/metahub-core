package tracing

import (
	"context"
	"os"
	"testing"
)

func TestSetupJeager(t *testing.T) {
	ctx := context.Background()
	jeagerC, err := SetupJeager(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer jeagerC.Terminate(ctx)
	if jeagerC.ThriftURI == "" {
		t.Fatal("expected non empty ThriftURI")
	}
}

func TestSetupJeagerENV(t *testing.T) {
	os.Setenv("JEAGER_PORT", "26686")
	ctx := context.Background()
	jeagerC, err := SetupJeager(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer jeagerC.Terminate(ctx)
	if jeagerC.ThriftURI == "" {
		t.Fatal("expected non empty ThriftURI")
	}
}
