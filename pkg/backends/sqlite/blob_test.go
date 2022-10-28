package sqlite

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func Test_BlobUpload_Create(t *testing.T) {
	bu := types.OCIBlobUpload{
		UploadedBy: "test",
		BlobType:   "layer",
		BlobID:     1,
		Repo:       "test",
	}
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

}
