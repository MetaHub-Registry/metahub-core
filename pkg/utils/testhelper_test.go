package utils

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestCreateHTTPTest(t *testing.T) {
	token := "token"
	testOrg := "qnib-pub-containers"
	testRepo := "qnib"
	testImage := "plain-featuretest"
	testTag := "20220715-amd64-generic"
	testProfile := "generic"
	testUser := "user"
	_, r := CreateNewHTTPTest(context.Background(), http.MethodGet, map[string]string{
		"repo":      testOrg,
		"repo2":     testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    testUser,
		"profileName": testProfile,
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{repo2}/{image}/manifests/{reference}", nil)
	assert.Equal(t, testOrg, chi.URLParam(r, "repo"))
}
