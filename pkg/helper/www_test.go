package helper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteResponse(t *testing.T) {
	w := httptest.NewRecorder()
	WriteResponse(w, http.StatusOK, []byte("test"))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test", w.Body.String())
}

func TestNewDetail(t *testing.T) {
	tests := []struct {
		name       string
		dType      string
		dName      string
		dAction    string
		wantAction string
		wantName   string
	}{
		{"GET maps to pull", "repository", "myrepo", "GET", "pull", "myrepo"},
		{"HEAD maps to pull", "repository", "myrepo", "HEAD", "pull", "myrepo"},
		{"PUT maps to pull", "repository", "myrepo", "PUT", "pull", "myrepo"},
		{"DELETE maps to del", "repository", "myrepo", "DELETE", "del", "myrepo"},
		{"empty name defaults to unknown", "repository", "", "GET", "pull", "unknown"},
		{"unknown action passes through", "repository", "myrepo", "CUSTOM", "CUSTOM", "myrepo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDetail(tc.dType, tc.dName, tc.dAction)
			assert.Equal(t, tc.dType, d.Type)
			assert.Equal(t, tc.wantName, d.Name)
			assert.Equal(t, tc.wantAction, d.Action)
		})
	}
}

func TestNewErrPayload(t *testing.T) {
	det := NewDetail("repository", "myrepo", "GET")
	ep := NewErrPayload("MANIFEST_UNKNOWN", "not found", det)
	assert.Equal(t, "MANIFEST_UNKNOWN", ep.Code)
	assert.Equal(t, "not found", ep.Message)
	require.Len(t, ep.Details, 1)
	assert.Equal(t, "myrepo", ep.Details[0].Name)
}

func TestPayloadToBytes(t *testing.T) {
	ep := NewErrPayload("DENIED", "unauthorized", Detail{})
	p := Payload{Errors: []ErrorPayload{ep}}
	b := p.ToBytes()
	require.NotEmpty(t, b)
	var parsed Payload
	err := json.Unmarshal(b, &parsed)
	require.NoError(t, err)
	assert.Equal(t, "DENIED", parsed.Errors[0].Code)
}

func TestWriteOCIErrror(t *testing.T) {
	w := httptest.NewRecorder()
	ep := NewErrPayload("MANIFEST_INVALID", "bad manifest", Detail{})
	WriteOCIErrror(w, http.StatusBadRequest, ep)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var parsed Payload
	err := json.Unmarshal(w.Body.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "MANIFEST_INVALID", parsed.Errors[0].Code)
}

func TestWriteOCIErrror_MultipleErrors(t *testing.T) {
	w := httptest.NewRecorder()
	ep1 := NewErrPayload("ERR1", "first", Detail{})
	ep2 := NewErrPayload("ERR2", "second", Detail{})
	WriteOCIErrror(w, http.StatusInternalServerError, ep1, ep2)
	var parsed Payload
	err := json.Unmarshal(w.Body.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Len(t, parsed.Errors, 2)
}
