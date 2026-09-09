package types

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// symlinkedOSReleaseTar mirrors the alpine >=3.21 layout: etc/os-release is a
// SYMLINK to ../usr/lib/os-release, which is the regular file. Regression
// fixture for pull-through manipulation of harbor/library/alpine:3.24, where
// EvalOS previously failed with "failed to find /etc/os-release".
func symlinkedOSReleaseTar(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "etc",
		Typeflag: tar.TypeDir,
	}))
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "etc/os-release",
		Typeflag: tar.TypeSymlink,
		Linkname: "../usr/lib/os-release",
	}))
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "usr/lib",
		Typeflag: tar.TypeDir,
	}))
	content := "NAME=\"Alpine Linux\"\nID=alpine\nVERSION_ID=3.24.0\n"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "usr/lib/os-release",
		Typeflag: tar.TypeReg,
		Size:     int64(len(content)),
	}))
	_, err := tw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func TestUncompressResolvesSymlinkedOSRelease(t *testing.T) {
	l := Layer{Data: symlinkedOSReleaseTar(t)}
	fs, err := l.Uncompress(context.Background(), "etc/os-release")
	require.NoError(t, err)
	_, found := fs["etc/os-release"]
	require.True(t, found, "symlinked etc/os-release should be materialized from its target")
}

func TestFindFileFollowsSymlink(t *testing.T) {
	l := Layer{Data: symlinkedOSReleaseTar(t)}
	cnt, found, err := l.FindFile(context.Background(), "etc/os-release")
	require.NoError(t, err)
	assert.True(t, found)
	os, err := ParseOS(cnt)
	require.NoError(t, err)
	assert.Equal(t, "alpine", os.ID)
}

func TestEvalOSFollowsSymlink(t *testing.T) {
	l := Layer{Data: symlinkedOSReleaseTar(t)}
	os, err := l.EvalOS(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "alpine", os.ID)
	assert.Equal(t, "Alpine Linux", os.Name)
}

// A garbage blob must surface the uncompress error — EvalOS used to mask it
// behind "failed to find /etc/os-release" (missing return after the error).
func TestEvalOSUncompressErrorPropagates(t *testing.T) {
	l := Layer{Data: []byte("not a tarball")}
	_, err := l.EvalOS(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uncompress")
	assert.NotContains(t, err.Error(), "failed to find")
}
