package types

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLayerUncompressStepByStep(t *testing.T) {
	ctx := context.Background()
	dt := NewDataTuple("data/layer/f1417ff83b319fbdae6dd9cd6d8c9c88002dcd75ecf6ec201c8c6894681cf2b5.dat")
	lFail := newLayerFromDT(dt)
	lFail.Data = []byte(``)
	_, err := lFail.Uncompress(ctx)
	if err == nil {
		t.Fatalf("Expected to fai")
	}
	l := newLayerFromDT(dt)
	fs, err := l.Uncompress(ctx)
	if err != nil {
		t.Fatalf("Failed to uncompress: %v", err)
	}
	if len(fs) != 88 {
		t.Fatalf("Expected one file, but got %d", len(fs))
	}
	cnt, found, err := l.FindFile(ctx, "etc/os-release")
	if err != nil {
		t.Fatalf("Failed to find file: %v", err)
	}
	if !found {
		t.Fatalf("Failed to find file")
	}
	os, err := ParseOS(cnt)
	if err != nil {
		t.Fatalf("Failed to find file: %v", err)
	}
	assert.Equal(t, "Alpine Linux", os.Name)
}

func TestLayerEvalOS(t *testing.T) {
	ctx := context.Background()
	dt := NewDataTuple("data/layer/f1417ff83b319fbdae6dd9cd6d8c9c88002dcd75ecf6ec201c8c6894681cf2b5.dat")
	l := newLayerFromDT(dt)
	os, err := l.EvalOS(ctx)
	if err != nil {
		t.Fatalf("Failed to eval OS: %v", err)
	}
	assert.Equal(t, "Alpine Linux", os.Name)
}
