package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/sekolahpintar/ews-worker/internal/middleware"
	"github.com/sekolahpintar/ews-worker/internal/model"
)

func injectClaims(claims *model.UserClaims, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), middleware.ClaimsContextKey(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func assertStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rr.Code != want {
		t.Errorf("status: got %d, want %d — body: %s", rr.Code, want, rr.Body.String())
	}
}

// ─── Process (batch EWS) authorization ─────────────────────────────────────

type stubbedProcessHandler struct{}

func (h *stubbedProcessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !claims.IsAdmin() {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func TestProcess_Forbidden(t *testing.T) {
	s10 := int64(10)
	tests := []*model.UserClaims{
		{Roles: []string{"siswa"}, SiswaID: &s10},
		{Roles: []string{"wali"}, WaliSiswaIDs: []int64{20}},
		{Roles: []string{"guru"}},
		{Roles: []string{"external"}},
	}
	for _, tc := range tests {
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler { return injectClaims(tc, next) })
		r.Post("/process", (&stubbedProcessHandler{}).ServeHTTP)
		req := httptest.NewRequest(http.MethodPost, "/process", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assertStatus(t, rr, http.StatusForbidden)
	}

	r := chi.NewRouter()
	r.Post("/process", (&stubbedProcessHandler{}).ServeHTTP)
	req := httptest.NewRequest(http.MethodPost, "/process", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assertStatus(t, rr, http.StatusUnauthorized)
}

func TestProcess_Allowed(t *testing.T) {
	tests := []*model.UserClaims{
		{Roles: []string{"admin"}},
		{Roles: []string{"superadmin"}},
		{Roles: []string{"staff"}},
	}
	for _, tc := range tests {
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler { return injectClaims(tc, next) })
		r.Post("/process", (&stubbedProcessHandler{}).ServeHTTP)
		req := httptest.NewRequest(http.MethodPost, "/process", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assertStatus(t, rr, http.StatusOK)
	}
}

// ─── ResolveAlert authorization ──────────────────────────────────────────

type stubbedResolveHandler struct{}

func (h *stubbedResolveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !claims.IsAdmin() && !claims.IsGuru() {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func TestResolveAlert_Forbidden(t *testing.T) {
	s10 := int64(10)
	tests := []*model.UserClaims{
		{Roles: []string{"siswa"}, SiswaID: &s10},
		{Roles: []string{"wali"}, WaliSiswaIDs: []int64{20}},
		{Roles: []string{"external"}},
	}
	for _, tc := range tests {
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler { return injectClaims(tc, next) })
		r.Patch("/alerts/1/resolve", (&stubbedResolveHandler{}).ServeHTTP)
		req := httptest.NewRequest(http.MethodPatch, "/alerts/1/resolve", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assertStatus(t, rr, http.StatusForbidden)
	}
}

func TestResolveAlert_Allowed(t *testing.T) {
	gid := int64(3)
	tests := []*model.UserClaims{
		{Roles: []string{"admin"}},
		{Roles: []string{"staff"}},
		{Roles: []string{"guru"}, GuruID: &gid},
	}
	for _, tc := range tests {
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler { return injectClaims(tc, next) })
		r.Patch("/alerts/1/resolve", (&stubbedResolveHandler{}).ServeHTTP)
		req := httptest.NewRequest(http.MethodPatch, "/alerts/1/resolve", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assertStatus(t, rr, http.StatusOK)
	}
}