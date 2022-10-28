package types

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/context"
)

type Request struct {
	Repo        string `json:"repo"`
	Tag         string `json:"tag"`
	Digest      string `json:"digest"`
	User        string `json:"user"`
	Profile     string `json:"profile"`
	Length      int    `json:"length"`
	Range       string `json:"range"`
	UUID        string `json:"uuid"`
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
	repo := filepath.Join(parRepo, parImg)
	user := "anonymous"
	profile := "generic"
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
	return Request{
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
