package types

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestNewRequest(t *testing.T) {
	ctx := context.Background()
	_, r := utils.CreateNewHTTPTest(http.MethodGet, map[string]string{}, map[string]string{
		"debug": "10",
	}, map[string]string{}, "/{repo}/{repo2}/{image}/manifests/{reference}", nil)
	req := NewRequest(ctx, r)
	assert.Equal(t, http.MethodGet, req.Method)
}

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

func TestExtractRepoName(t *testing.T) {
	ctx := context.Background()
	fnc := "TestExtractRepoName"
	testRepo := "qnib"
	testImage := "plain-featuretest"
	testTag := "20220715-amd64-generic"
	testProfile := "generic"
	token := "token"
	debug := 0
	c, span := tracer.Start(ctx, fnc)
	defer span.End()
	_ = c
	req := EmptyRequest(0, 5)
	_, r := utils.CreateNewHTTPTest(http.MethodHead, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    "qnib",
		"profileName": testProfile,
		"debug":       fmt.Sprintf("%d", debug),
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/blobs/{reference}", nil)
	req.ExtractRepoName(span, r)
	assert.Equal(t, req.Repo, fmt.Sprintf("%s/%s", testRepo, testImage))
	assert.Equal(t, req.Tag, testTag)
}

func TestParsePagination(t *testing.T) {
	ctx := context.Background()
	fnc := "TestParsePagination"
	testRepo := "qnib"
	testImage := "plain-featuretest"
	testTag := "20220715-amd64-generic"
	testProfile := "generic"
	token := "token"
	debug := 0
	c, span := tracer.Start(ctx, fnc)
	defer span.End()
	_ = c
	req := EmptyRequest(0, 5)
	_, r := utils.CreateNewHTTPTest(http.MethodHead, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    "qnib",
		"profileName": testProfile,
		"debug":       fmt.Sprintf("%d", debug),
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/blobs/{reference}?n=100&last=tag123", nil)
	req.ParsePagination(span, r)
	assert.Equal(t, 100, req.Number)
	assert.Equal(t, "tag123", req.Last)
}

func TestParseProto(t *testing.T) {
	ctx := context.Background()
	fnc := "TestParseProto"
	c, span := tracer.Start(ctx, fnc)
	defer span.End()
	_ = c
	req := EmptyRequest(0, 5)
	_, r := utils.CreateNewHTTPTest(http.MethodHead, map[string]string{}, map[string]string{}, map[string]string{}, "http://localhost/{repo}/{image}/blobs/{reference}?n=100&last=tag123", nil)
	req.ParseProto(span, r)
	assert.Equal(t, "http", req.Protocol)
	r.Proto = "HTTP/1.0"
	req.ParseProto(span, r)
	assert.Equal(t, "http", req.Protocol)
	r.Proto = "HTTP/1.1"
	req.ParseProto(span, r)
	assert.Equal(t, "http", req.Protocol)
	r.Proto = "HTTP/2.0"
	req.ParseProto(span, r)
	assert.Equal(t, "https", req.Protocol)
	req = EmptyRequest(0, 5)
	r.Proto = "HTTP/3.0"
	req.ParseProto(span, r)
	assert.Equal(t, "", req.Protocol)
}

func TestParseProfile(t *testing.T) {
	ctx := context.Background()
	fnc := "TestParseProfile"
	testProfile := "generic"
	token := "token"
	debug := 0
	c, span := tracer.Start(ctx, fnc)
	defer span.End()
	_ = c
	req := EmptyRequest(0, 5)
	_, r := utils.CreateNewHTTPTest(http.MethodGet, map[string]string{}, map[string]string{
		"accountName": token,
		"userName":    "qnib/generic",
		//"profileName": testProfile,
		"debug": fmt.Sprintf("%d", debug),
	}, map[string]string{}, "/{repo}/{image}/blobs/{reference}", nil)
	prf, usr, err := req.ParseProfile(span, r)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "qnib", usr)
	assert.Equal(t, testProfile, prf)

	_, r = utils.CreateNewHTTPTest(http.MethodGet, map[string]string{}, map[string]string{
		"accountName": token,
		"profileName": testProfile,
		"debug":       fmt.Sprintf("%d", debug),
	}, map[string]string{}, "/{repo}/{image}/blobs/{reference}", nil)
	prf, usr, err = req.ParseProfile(span, r)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, testProfile, prf)
	assert.Equal(t, "", usr)
	// Fail
	_, r = utils.CreateNewHTTPTest(http.MethodGet, map[string]string{}, map[string]string{
		"accountName": token,
		"debug":       fmt.Sprintf("%d", debug),
	}, map[string]string{}, "/{repo}/{image}/blobs/{reference}", nil)
	prf, usr, err = req.ParseProfile(span, r)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "", prf)
	assert.Equal(t, "", usr)
}

func TestParseAuth(t *testing.T) {
	ctx := context.Background()
	fnc := "TestParseAuth"
	testRepo := "qnib"
	testImage := "plain-featuretest"
	testTag := "20220715-amd64-generic"
	testProfile := "generic"
	token := "token"
	debug := 0
	c, span := tracer.Start(ctx, fnc)
	defer span.End()
	_ = c
	req := EmptyRequest(0, 5)
	_, r := utils.CreateNewHTTPTest(http.MethodHead, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    "qnib",
		"profileName": testProfile,
		"user":        "user",
		"debug":       fmt.Sprintf("%d", debug),
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/blobs/{reference}", nil)
	req.ParseAuth(span, r)
	assert.Equal(t, "qnib", req.User)
	assert.Equal(t, "user", req.UserRole)
	_, r = utils.CreateNewHTTPTest(http.MethodHead, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"profileName": testProfile,
		"user":        "user",
		"debug":       fmt.Sprintf("%d", debug),
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/blobs/{reference}", nil)
	req.ParseAuth(span, r)
	assert.Equal(t, "anonymous", req.User)
	assert.Equal(t, "user", req.UserRole)
}

func TestParseRange(t *testing.T) {
	ctx := context.Background()
	fnc := "TestParseRange"
	testRepo := "qnib"
	testImage := "plain-featuretest"
	testTag := "20220715-amd64-generic"
	testProfile := "generic"
	token := "token"
	debug := 0
	c, span := tracer.Start(ctx, fnc)
	defer span.End()
	_ = c
	req := EmptyRequest(0, 5)
	_, r := utils.CreateNewHTTPTest(http.MethodHead, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    "qnib",
		"profileName": testProfile,
		"debug":       fmt.Sprintf("%d", debug),
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
		"Content-Range": "100-200",
	}, "/{repo}/{image}/blobs/{reference}", nil)
	req.ParseRange(span, r)
	assert.Equal(t, req.RangeBytes, int64(200))
	assert.Equal(t, req.Range, "100-200")
	assert.Equal(t, req.RangeStartByte, int64(100))
}
