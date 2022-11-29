package sqlite

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_BlobUpload_Create(t *testing.T) {
	bs := utils.NewBootstrap()
	defer TeardownTest(bs)
	bu, l0Buf := types.FT_arm_BlobUpload_Layer0()
	bu.RangeStart = 0
	bu.RangeStop = int64(len(l0Buf) - 1)
	err := sq.BlobUpload.Create(&bu)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, bu.SqlID)
	assert.NotEqual(t, "", bu.UUID)
	rr := httptest.NewRecorder()
	sq.BlobUpload.WriteOCICreateResponse(rr, &bu)
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusCreated)
	}
	loc := fmt.Sprintf("/v2/%s/blobs/uploads/%s", bu.Repo, bu.UUID)
	assert.Equal(t, loc, rr.Header().Get("Location"))
	assert.Equal(t, bu.UUID.String(), rr.Header().Get("Docker-Upload-UUID"))
	gotBu, err := sq.BlobUpload.GetByUUID(bu.UUID.String())
	assert.NoError(t, err)
	assert.Equal(t, bu.UUID, gotBu.UUID)
	assert.Equal(t, bu.UploadedBy, gotBu.UploadedBy)
	assert.Equal(t, bu.Location, gotBu.Location)
	// Accepted Response
	rr = httptest.NewRecorder()
	sq.BlobUpload.WriteOCIAcceptedResponse(rr, &bu)
	if status := rr.Code; status != http.StatusAccepted {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusAccepted)
	}
	assert.Equal(t, "0", rr.Header().Get("Content-Length"))
	assert.Equal(t, loc, rr.Header().Get("Location"))
	assert.Equal(t, bu.UUID.String(), rr.Header().Get("Docker-Upload-UUID"))
	// PatchResponse
	rr = httptest.NewRecorder()
	sq.BlobUpload.WriteOCIPatchResponse(rr, &bu)
	if status := rr.Code; status != http.StatusAccepted {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusAccepted)
	}
	assert.Equal(t, bu.Location, rr.Header().Get("Location"))
	// PutResponse
	rr = httptest.NewRecorder()
	sq.BlobUpload.WriteOCIPutResponse(rr, &bu)
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusCreated)
	}
	assert.Equal(t, "0", rr.Header().Get("Content-Length"))
	assert.Equal(t, bu.Digest, rr.Header().Get("Docker-Content-Digest"))
}
