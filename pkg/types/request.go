package types

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	gCtx "github.com/gorilla/context"
	"go.opentelemetry.io/otel/trace"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

var (
	userReg    = regexp.MustCompile(`(?P<user>[\w\-\.\_]+)(\/(?P<profile>[\/\w\-\.\_#]+))?`)
	pkgNameReq = "types/request"
)

const (
	debugLevel = 4
)

type Request struct {
	Number         int    `json:"number"`
	Last           string `json:"last"`
	Host           string `json:"host"`
	Protocol       string `json:"protocol"`
	Repo           string `json:"repo"`
	Tag            string `json:"tag"`
	Digest         string `json:"digest"`
	Platform       string `json:"platform"`
	UserRole       string `json:"user_role"`
	User           string `json:"user"`
	Profile        string `json:"profile"`
	Length         int    `json:"length"`
	Range          string `json:"range"`
	RangeStartByte int64  `json:"start_byte"`
	RangeBytes     int64  `json:"bytes"`
	UUID           string `json:"uuid"`
	Method         string `json:"method"`
	Scope          string `json:"scope"`
	ContentType    string `json:"content-type"`
	DebugLevel     int    `json:"debug_level"`
}

func NewRequest(ctx context.Context, r *http.Request) Request {
	debug := 0
	vD := gCtx.Get(r, "debug")
	if vD != nil {
		debug = vD.(int)
	}
	var span trace.Span
	var l *logrus.Entry
	if debug > debugLevel {
		l, ctx, span = utils.InitLogging(ctx, tracer, map[string]any{
			"package": pkgNameReq,
			"func":    "NewRequest",
		})
		defer span.End()
	}
	utils.Debug(l, span, "Repo from URLParam: %s", chi.URLParam(r, "repo"))

	//dumpedReq, _ := httputil.DumpRequest(r, false)
	//logrus.Errorf("Request:#### \n%s\n####", dumpedReq)
	req := Request{
		Number:     0,
		DebugLevel: debug,
	}
	parRepo, parImg, parRef := extractRepoName(r)
	//utils.Debug(l, span, "repo:%s", parRepo)
	var tag string
	dig := r.FormValue("digest")
	if strings.HasPrefix(parRef, "sha256:") {
		dig = parRef
	} else {
		tag = parRef
	}
	// ENable pagination (n=100&last=tag123)
	nStr := r.URL.Query().Get("n")
	if nStr != "" {
		n, err := strconv.Atoi(nStr)
		if err != nil {
			logrus.Errorf("Failed to convert nStr: %s", err)
		}
		req.Number = n
	}
	req.Last = r.FormValue("last")
	switch r.Proto {
	case "HTTP/1.0":
		req.Protocol = "http"
	case "HTTP/1.1":
		req.Protocol = "http"
	case "HTTP/2.0":
		req.Protocol = "https"
	}

	srvAddrVal := ctx.Value(http.LocalAddrContextKey)
	srvAddr := ""
	if srvAddrVal != nil {
		srvAddr = srvAddrVal.(net.Addr).String()
	} else {
		logrus.Debugf("Not able to derive the server address from the context")
		srvAddr = r.Host
	}
	req.Host = srvAddr
	repo := filepath.Join(parRepo, parImg)
	user := "anonymous"
	userRole := "user"
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
	vUR := gCtx.Get(r, "userRole")
	if vUR != nil {
		userRole = vUR.(string)
	} else {
		utils.Debug(l, span, "No userRole found in context")
	}
	vU := gCtx.Get(r, "userName")
	if vU != nil {
		user = vU.(string)
	}
	vP := gCtx.Get(r, "profileName")
	if vP != nil {
		profile = vP.(string)
		/*
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", fmt.Sprintf("found profileName '%s' in request,", profile)),
			))
		*/
	} else {
		m, err := utils.ReSubMatchMap(userReg, user)
		if err != nil {
			logrus.Errorf("Failed to extract user from: %s", user)
		}
		if _, ok := m["user"]; !ok {
			logrus.Errorf("Failed to extract user from: %s", user)
		} else {
			user = m["user"]
			// FIXME: Why is the profileName not carried over?
			if prf, ok := m["profile"]; ok {
				if prf != "" {
					profile = prf
					/*
						span.AddEvent("log", trace.WithAttributes(
							attribute.String("log.severity", "debug"),
							attribute.String("log.message", fmt.Sprintf("found profile '%s' encoded in username '%s'", profile, user)),
						))
					*/
				}
			}
		}
	}
	/*
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", fmt.Sprintf("user:'%s' / profile:'%s'", user, profile)),
		))
	*/
	rnge := r.Header.Get("Content-Range")
	req.RangeStartByte = -1
	req.RangeBytes = -1
	if rnge != "" {
		req.Range = rnge
		s := strings.Split(rnge, "-")
		if len(s) == 2 {
			i, err := strconv.Atoi(s[0])
			if err != nil {
				logrus.Errorf("Failed to convert range: %s", err)
			}
			req.RangeStartByte = int64(i)
			iB, err := strconv.Atoi(s[1])
			if err != nil {
				logrus.Errorf("Failed to convert range: %s", err)
			}
			req.RangeBytes = int64(iB)
		}
	}
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
	req.Method = r.Method
	req.Scope = scope
	req.Repo = repo
	req.Tag = tag
	req.Digest = dig
	req.User = user
	req.UserRole = userRole
	req.Profile = profile
	req.Length = cntLen
	req.Range = rnge
	req.UUID = uid
	req.ContentType = cntType
	return req
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
	repo2 := chi.URLParam(r, "repo2")
	if repo2 != "" {
		repo = filepath.Join(repo, repo2)
	}
	repo3 := chi.URLParam(r, "repo3")
	if repo3 != "" {
		repo = filepath.Join(repo, repo3)
	}
	img = chi.URLParam(r, "image")
	ref = chi.URLParam(r, "reference")
	return
}
