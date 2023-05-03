package types

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_ExtractFromRequest(t *testing.T) {
	ctx := context.Background()
	token := "token"
	testOrg := "qnib-pub-containers"
	testRepo := "qnib"
	testImage := "plain-featuretest"
	testTag := "20220715-amd64-generic"
	testProfile := "generic"
	testUser := "user"
	_, r := utils.CreateNewHTTPTest(http.MethodGet, map[string]string{
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
	repo, img, ref := extractRepoName(r)
	assert.Equal(t, fmt.Sprintf("%s/%s", testOrg, testRepo), repo)
	assert.Equal(t, testImage, img)
	assert.Equal(t, testTag, ref)
	req := NewRequest(ctx, r)
	assert.Equal(t, fmt.Sprintf("%s/%s/%s", testOrg, testRepo, testImage), req.Repo)
	assert.Equal(t, testUser, req.User)
	assert.Equal(t, testProfile, req.Profile)

}
