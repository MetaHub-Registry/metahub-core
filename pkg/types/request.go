package types

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/context"
	"github.com/sirupsen/logrus"
)

type Request struct {
	Host        string `json:"host"`
	Repo        string `json:"repo"`
	Tag         string `json:"tag"`
	Digest      string `json:"digest"`
	Platform    string `json:"platform"`
	User        string `json:"user"`
	Profile     string `json:"profile"`
	Length      int    `json:"length"`
	Range       string `json:"range"`
	UUID        string `json:"uuid"`
	Method      string `json:"method"`
	Scope       string `json:"scope"`
	ContentType string `json:"content-type"`
}

func NewRequest(r *http.Request) Request {
	//dumpedReq, _ := httputil.DumpRequest(r, false)
	//logrus.Errorf("Request:#### \n%s\n####", dumpedReq)
	parRepo, parImg, parRef := extractRepoName(r)
	var tag string
	dig := r.FormValue("digest")
	if strings.HasPrefix(parRef, "sha256:") {
		dig = parRef
	} else {
		tag = parRef
	}
	host := strings.Split(r.Host, ":")[0]
	repo := filepath.Join(parRepo, parImg)
	user := "anonymous"
	profile := "generic"
	authorizationHeader := r.Header.Get("authorization")
	if authorizationHeader != "" {
		bearerToken := strings.Split(authorizationHeader, " ")
		if len(bearerToken) != 2 {
			logrus.Errorf("invalid authorization header: %q", authorizationHeader)
		}
		accessTokenString := bearerToken[1]
		decoded, _ := base64.StdEncoding.DecodeString(accessTokenString)
		parts := strings.Split(string(decoded), ":")
		if len(parts) != 2 {
			logrus.Errorf("invalid access token: %q", accessTokenString)
		} else {
			user = parts[0]
		}
	}
	vU := context.Get(r, "userName")
	if vU != nil {
		user = vU.(string)
	}
	vP := context.Get(r, "profileName")
	if vP != nil {
		profile = vP.(string)
	}
	rnge := r.Header.Get("Content-Range")
	uid := chi.URLParam(r, "uuid")
	cntLenStr := r.Header.Get("Content-Length")
	cntType := r.Header.Get("Content-Type")
	cntLen, _ := strconv.Atoi(cntLenStr)
	scope := ""
	rManifest, _ := regexp.Compile(`^\/v2\/[\w\/]+\/manifests\/\w+`)
	rBlob, _ := regexp.Compile(`^\/v2\/[\w\/]+\/blobs\/uploads`)
	switch {
	// Many methods are available on these structs. Here's
	// a match test like we saw earlier.
	case rManifest.MatchString(r.URL.Path):
		scope = "manifests"
	case rBlob.MatchString(r.URL.Path):
		scope = "blobs"
	default:
		scope = "unknown"
	}
	return Request{
		Method:      r.Method,
		Scope:       scope,
		Host:        host,
		Repo:        repo,
		Tag:         tag,
		Digest:      dig,
		User:        user,
		Profile:     profile,
		Length:      cntLen,
		Range:       rnge,
		UUID:        uid,
		ContentType: cntType,
	}
}

func (req Request) String() string {
	res, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return ""
	}
	return string(res)
}

func extractRepoName(r *http.Request) (repo, img, ref string) {
	repo = chi.URLParam(r, "repo")
	img = chi.URLParam(r, "image")
	ref = chi.URLParam(r, "reference")
	return
}
