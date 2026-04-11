package types

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBlobReferenceFromBytes(t *testing.T) {
	data := []byte(`{"digest":"sha256:abc123","mediaType":"application/octet-stream","size":42}`)
	br, err := NewBlobReferenceFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "sha256:abc123", br.Digest)
	assert.Equal(t, "application/octet-stream", br.MediaType)
	assert.Equal(t, int64(42), br.Size)
	assert.Equal(t, data, br.Data)
}

func TestNewBlobReferenceFromBytes_InvalidJSON(t *testing.T) {
	_, err := NewBlobReferenceFromBytes([]byte(`{invalid`))
	assert.Error(t, err)
}

func TestBlobReferenceToDescriptor(t *testing.T) {
	br := BlobReference{
		Digest:    "sha256:abc123",
		MediaType: "application/octet-stream",
		Size:      42,
	}
	desc := br.ToDescriptor()
	assert.Equal(t, "sha256:abc123", desc.Digest.String())
	assert.Equal(t, "application/octet-stream", desc.MediaType)
	assert.Equal(t, int64(42), desc.Size)
}

func TestBlobReferenceWriteWWW(t *testing.T) {
	br := BlobReference{
		Digest:    "sha256:abc123",
		MediaType: "application/octet-stream",
		Data:      []byte("hello"),
		Size:      5,
	}
	w := httptest.NewRecorder()
	br.WriteWWW(w)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "sha256:abc123", w.Header().Get("Docker-Content-Digest"))
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "hello", w.Body.String())
}

func TestBlobToDescriptor(t *testing.T) {
	b := Blob{
		Digest:    "sha256:def456",
		MediaType: "application/vnd.oci.image.config.v1+json",
		Size:      100,
	}
	desc := b.ToDescriptor()
	assert.Equal(t, "sha256:def456", desc.Digest.String())
	assert.Equal(t, int64(100), desc.Size)
}

func TestBlobWriteWWW(t *testing.T) {
	b := Blob{
		Digest:    "sha256:def456",
		MediaType: "application/octet-stream",
		Data:      []byte("blobdata"),
	}
	w := httptest.NewRecorder()
	b.WriteWWW(w)
	assert.Equal(t, "sha256:def456", w.Header().Get("Docker-Content-Digest"))
	assert.Equal(t, "blobdata", w.Body.String())
}

func TestBlobWriteWWWHeader(t *testing.T) {
	b := Blob{Digest: "sha256:head123"}
	w := httptest.NewRecorder()
	b.WriteWWWHeader(w)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "sha256:head123", w.Header().Get("Docker-Content-Digest"))
	assert.Empty(t, w.Body.String(), "HEAD should have no body")
}
