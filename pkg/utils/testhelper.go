package utils

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi/v5"
	gCtx "github.com/gorilla/context"
)

func CreateNewHTTPTest(meth string, p, c, h map[string]string, url string, b []byte) (w httptest.ResponseRecorder, r *http.Request) {
	w = *httptest.NewRecorder()
	r = httptest.NewRequest(meth, url, bytes.NewReader(b))
	rctx := chi.NewRouteContext()
	for k, v := range p {
		rctx.URLParams.Add(k, v)
	}
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	for k, v := range c {
		gCtx.Set(r, k, v)
	}
	for k, v := range h {
		r.Header.Set(k, v)
	}
	r.Header.Set("Content-Length", fmt.Sprintf("%d", len(b)))
	return
}
