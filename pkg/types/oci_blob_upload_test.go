package types

import (
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestOCIbuWWW(t *testing.T) {
	uid0, _ := uuid.FromBytes([]byte("1234567890"))
	w0 := httptest.NewRecorder()
	b0 := OCIBlobUpload{
		UUID:     uid0,
		Location: "Loc0",
	}
	// WriteOCIAcceptedResponse
	b0.WriteOCIAcceptedResponse(w0)
	assert.Equal(t, 202, w0.Code)
	assert.Equal(t, "Loc0", w0.Header().Get("Location"))
	assert.NotEqual(t, "", b0.UUID.String())
	assert.Equal(t, b0.UUID.String(), w0.Header().Get("Docker-Upload-UUID"))
	assert.Equal(t, "0", w0.Header().Get("Content-Length"))
	// WriteOCICreateResponse
	w1 := httptest.NewRecorder()
	b1 := OCIBlobUpload{
		UUID:     uid0,
		Location: "Loc1",
	}
	b1.WriteOCICreateResponse(w1)
	assert.Equal(t, 201, w1.Code)
	assert.Equal(t, "Loc1", w1.Header().Get("Location"))
	assert.NotEqual(t, "", b1.UUID.String())
	assert.Equal(t, b1.UUID.String(), w1.Header().Get("Docker-Upload-UUID"))
	assert.Equal(t, "0", w1.Header().Get("Content-Length"))
	// WriteOCIPatchResponse
	w2 := httptest.NewRecorder()
	b2 := OCIBlobUpload{
		RangeStart: 0,
		RangeStop:  10,
		UUID:       uid0,
		Location:   "Loc2",
	}
	b2.WriteOCIPatchResponse(w2)
	assert.Equal(t, 202, w2.Code)
	assert.Equal(t, "Loc2", w2.Header().Get("Location"))
	assert.NotEqual(t, "", b2.UUID.String())
	assert.Equal(t, b2.UUID.String(), w2.Header().Get("Docker-Upload-UUID"))
	assert.Equal(t, "0-10", w2.Header().Get("Range"))
	assert.Equal(t, "0", w2.Header().Get("Content-Length"))
	// WriteOCIPutResponse
	w3 := httptest.NewRecorder()
	b3 := OCIBlobUpload{
		RangeStart: 0,
		RangeStop:  10,
		UUID:       uid0,
		Location:   "Loc3",
		Digest:     "sha256:1234567890",
	}
	b3.WriteOCIPutResponse(w3)
	assert.Equal(t, 201, w3.Code)
	assert.Equal(t, "Loc3", w3.Header().Get("Location"))
	assert.NotEqual(t, "", b3.UUID.String())
	assert.Equal(t, "sha256:1234567890", w3.Header().Get("Docker-Content-Digest"))
	assert.Equal(t, "0", w3.Header().Get("Content-Length"))
	assert.Equal(t, "0-10", w2.Header().Get("Range"))

}
