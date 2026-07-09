package types

import (
	"context"
	"testing"
)

func TestRequestedModelContext(t *testing.T) {
	ctx := context.Background()
	if got := RequestedModelFromContext(ctx); got != "" {
		t.Fatalf("unset context should yield \"\", got %q", got)
	}
	ctx = WithRequestedModel(ctx, "qnib/minicpm5")
	if got := RequestedModelFromContext(ctx); got != "qnib/minicpm5" {
		t.Fatalf("RequestedModelFromContext = %q, want qnib/minicpm5", got)
	}
	// Independent of the quant key.
	ctx = WithRequestedQuant(ctx, "q8-0")
	if got := RequestedModelFromContext(ctx); got != "qnib/minicpm5" {
		t.Fatalf("model must survive alongside quant, got %q", got)
	}
	if got := RequestedQuantFromContext(ctx); got != "q8-0" {
		t.Fatalf("quant = %q, want q8-0", got)
	}
}
