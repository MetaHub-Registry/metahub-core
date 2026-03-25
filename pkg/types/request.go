package types

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	gCtx "github.com/gorilla/context"
	"go.opentelemetry.io/otel/attribute"
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
	logSev         = "log.severity"
	debugThreshold = 4
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
	DebugThreshold int    `json:"debug_threshold"`
	DebugLevel     int    `json:"debug_level"`
}

func EmptyRequest(no, debLvl int) Request {
	return Request{
		Number:         no,
		DebugThreshold: debugThreshold,
		DebugLevel:     debLvl,
	}
}

func (req *Request) ExtractRepoName(span trace.Span, r *http.Request) (err error) {
	parRepo, parImg, parRef := extractRepoName(r)
	//utils.Debug(l, span, "repo:%s", parRepo)
	var tag string
	dig := r.FormValue("digest")
	if strings.HasPrefix(parRef, "sha256:") {
		dig = parRef
	} else {
		tag = parRef
	}
	req.Log(span, "Extracted from URL - repo:"+parRepo+" / image:"+parImg+" / reference:"+parRef+" / digest:"+dig)
	repo := filepath.Join(parRepo, parImg)
	req.Repo = repo
	req.Tag = tag
	req.Digest = dig
	return nil
}

func (req *Request) ParsePagination(span trace.Span, r *http.Request) (err error) {
	// ENable pagination (n=100&last=tag123)
	nStr := r.URL.Query().Get("n")
	if nStr != "" {
		n, err := strconv.Atoi(nStr)
		if err != nil {
			logrus.Errorf("Failed to convert nStr: %s", err)
			return err
		}
		req.Number = n
	}
	last := r.FormValue("last")
	if last == "" {
		last = r.URL.Query().Get("last")
	}
	req.Last = last
	return nil
}

func (req *Request) ParseProto(span trace.Span, r *http.Request) (err error) {
	switch r.Proto {
	case "HTTP/1.0":
		req.Protocol = "http"
	case "HTTP/1.1":
		req.Protocol = "http"
	case "HTTP/2.0":
		req.Protocol = "https"
	}
	return nil
}

func (req Request) Log(span trace.Span, format string, a ...any) {
	if span == nil {
		return
	}
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(logSev, "debug"),
		attribute.String("log.message", fmt.Sprintf(format, a...)),
	))
}

func (req *Request) ParseProfile(span trace.Span, r *http.Request) (profile, user string, err error) {
	vU := gCtx.Get(r, "userName")
	if vU != nil {
		user = vU.(string)
	}

	vP := gCtx.Get(r, "profileName")
	if vP != nil {
		profile = vP.(string)
		req.Log(span, "found profileName '%s' in request,", profile)
	} else {
		m, err := utils.ReSubMatchMap(userReg, user)
		if err != nil {
			logrus.Errorf("Failed to extract user from: '%s'", user)
		}
		if _, ok := m["user"]; !ok {
			logrus.Errorf("Failed to extract user from: '%s'", user)
		} else {
			user = m["user"]
			if prf, ok := m["profile"]; ok {
				if prf != "" {
					profile = prf
				}
			}
		}
	}
	return profile, user, nil
}

func (req *Request) ParseAuth(span trace.Span, r *http.Request) (err error) {
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

	vPrf, vUser, _ := req.ParseProfile(span, r)
	if vPrf != "" {
		profile = vPrf
	}
	if vUser != "" {
		user = vUser
	}

	// Read userRole from gorilla context (set by OCIAuthMiddleware)
	vRole := gCtx.Get(r, "userRole")
	if vRole != nil {
		if role, ok := vRole.(string); ok && role != "" {
			userRole = role
		}
	}

	req.User = user
	req.UserRole = userRole
	req.Profile = profile

	return nil
}

func (req *Request) ParseRange(span trace.Span, r *http.Request) (err error) {
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
	return nil
}

func NewRequest(ctx context.Context, r *http.Request) Request {
	debug := 0
	vD := gCtx.Get(r, "debug")
	switch v := vD.(type) {
	case string:
		debug, _ = strconv.Atoi(v)
	case int:
		debug = v
	}
	var span trace.Span
	var l *logrus.Entry
	if debug > debugThreshold {
		l, ctx, span = utils.InitLogging(ctx, tracer, map[string]any{
			"package": pkgNameReq,
			"func":    "NewRequest",
		})
		defer span.End()
	}
	utils.Debug(l, span, "Repo from URLParam: %s", chi.URLParam(r, "repo"))

	//dumpedReq, _ := httputil.DumpRequest(r, false)
	//logrus.Errorf("Request:#### \n%s\n####", dumpedReq)
	req := EmptyRequest(0, debug)
	req.Method = r.Method
	req.Log(span, "Starting NewRequest processing for %s %s", r.Method, r.URL.Path)
	req.ExtractRepoName(span, r)
	req.ParsePagination(span, r)
	req.ParseProto(span, r)

	srvAddrVal := ctx.Value(http.LocalAddrContextKey)
	srvAddr := ""
	if srvAddrVal != nil {
		srvAddr = srvAddrVal.(net.Addr).String()
	} else {
		logrus.Debugf("Not able to derive the server address from the context")
		srvAddr = r.Host
	}
	req.Host = srvAddr

	req.ParseRange(span, r)
	req.ParseAuth(span, r)
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
	if debug > debugThreshold {
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(logSev, "debug"),
			attribute.String("method", r.Method),
			attribute.Int("content-length", cntLen),
			attribute.String("content-type", cntType),
			attribute.String("uuid", uid),
			attribute.String("protocol", req.Protocol),
			attribute.String("host", req.Host),
			attribute.Int("number", req.Number),
			attribute.String("last", req.Last),
		))
	}
	uRole := gCtx.Get(r, "userRole")
	if uRole != nil {
		req.UserRole = uRole.(string)
	}
	uName := gCtx.Get(r, "userName")
	if uName != nil {
		req.User = uName.(string)
	}
	req.Method = r.Method
	req.Scope = scope
	req.Length = cntLen
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
