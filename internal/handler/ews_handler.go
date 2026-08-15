package handler

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/sekolahpintar/ews-worker/internal/middleware"
	"github.com/sekolahpintar/ews-worker/internal/repository"
	"github.com/sekolahpintar/ews-worker/internal/service"
)

type EWSHandler struct {
	svc          *service.EWSService
	processMu    sync.Mutex
	isProcessing bool
}

func NewEWSHandler(svc *service.EWSService) *EWSHandler {
	return &EWSHandler{svc: svc}
}

// Process POST /api/v1/ews/process — trigger full batch EWS processing.
// Permission: operational roles only (admin/superadmin/staff). Non-authorized roles get 403.
// Concurrency guard: max 1 concurrent batch run.
func (h *EWSHandler) Process(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		jsonForbidden(w)
		return
	}
	if !claims.IsAdmin() {
		jsonForbidden(w)
		return
	}

	h.processMu.Lock()
	if h.isProcessing {
		h.processMu.Unlock()
		jsonError(w, http.StatusTooManyRequests, "EWS batch processing is already running")
		return
	}
	h.isProcessing = true
	h.processMu.Unlock()

	defer func() {
		h.processMu.Lock()
		h.isProcessing = false
		h.processMu.Unlock()
	}()

	result, err := h.svc.ProcessAll(r.Context())
	if err != nil {
		jsonServerError(w, err.Error())
		return
	}
	jsonOK(w, result, "EWS batch processing complete")
}

// ProcessSiswa POST /api/v1/ews/process-siswa/{siswaId} — process a single student.
// Permission: admin/staff or assigned teacher (CanReadSiswa).
func (h *EWSHandler) ProcessSiswa(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		jsonForbidden(w)
		return
	}

	siswaID, err := strconv.ParseInt(chi.URLParam(r, "siswaId"), 10, 64)
	if err != nil {
		jsonBadRequest(w, "invalid siswaId")
		return
	}

	if !claims.CanReadSiswa(siswaID) {
		jsonForbidden(w)
		return
	}

	if err := h.svc.ProcessSiswa(r.Context(), siswaID); err != nil {
		if err == repository.ErrNotFound {
			jsonNotFound(w, "siswa tidak ditemukan")
			return
		}
		jsonServerError(w, err.Error())
		return
	}
	jsonOK(w, nil, "EWS check complete")
}

// ListAlerts GET /api/v1/ews/alerts — paginated list of EWS alerts.
// Scope injection: siswa auto-scoped to own ID; wali scoped to ward IDs; guru/admin see all.
func (h *EWSHandler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		jsonForbidden(w)
		return
	}

	q := r.URL.Query()
	filters := map[string]string{
		"mst_siswa_id": q.Get("mst_siswa_id"),
		"is_resolved":  q.Get("is_resolved"),
		"level":        q.Get("level"),
		"kategori":     q.Get("kategori"),
	}

	// Enforce scope for non-admin/guru
	if !claims.IsAdmin() && !claims.IsGuru() {
		requestedID := filters["mst_siswa_id"]
		if claims.IsSiswa() {
			if claims.SiswaID == nil {
				jsonForbidden(w)
				return
			}
			ownID := strconv.FormatInt(*claims.SiswaID, 10)
			if requestedID != "" && requestedID != ownID {
				jsonForbidden(w)
				return
			}
			filters["mst_siswa_id"] = ownID
		} else if claims.IsWali() {
			if requestedID == "" {
				jsonForbidden(w)
				return
			}
			sid, err := strconv.ParseInt(requestedID, 10, 64)
			if err != nil || !claims.CanReadSiswa(sid) {
				jsonForbidden(w)
				return
			}
		} else {
			jsonForbidden(w)
			return
		}
	}

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if perPage < 1 {
		perPage = 20
	}

	list, total, err := h.svc.ListAlerts(r.Context(), filters, page, perPage)
	if err != nil {
		jsonServerError(w, err.Error())
		return
	}
	jsonPaginated(w, list, total, page, perPage, "OK")
}

// GetAlertsBySiswa GET /api/v1/ews/alerts/{siswaId} — all alerts for a student.
// Permission: caller must have CanReadSiswa access.
func (h *EWSHandler) GetAlertsBySiswa(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		jsonForbidden(w)
		return
	}

	siswaID, err := strconv.ParseInt(chi.URLParam(r, "siswaId"), 10, 64)
	if err != nil {
		jsonBadRequest(w, "invalid siswaId")
		return
	}

	if !claims.CanReadSiswa(siswaID) {
		jsonForbidden(w)
		return
	}

	list, err := h.svc.GetAlertsBySiswa(r.Context(), siswaID)
	if err != nil {
		jsonServerError(w, err.Error())
		return
	}
	jsonOK(w, list, "OK")
}

// ResolveAlert PATCH /api/v1/ews/alerts/{id}/resolve — manually resolve an alert.
// Permission: admin/staff/guru only. Siswa/wali forbidden (403).
// Guru: verifies access to the student owning the alert before resolving.
// Server-side actor: records claims.UserID.
func (h *EWSHandler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || (!claims.IsAdmin() && !claims.IsGuru()) {
		jsonForbidden(w)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonBadRequest(w, "invalid id")
		return
	}

	// Guru role: fetch alert to verify ownership/access to the student
	if !claims.IsAdmin() {
		alert, fetchErr := h.svc.FindAlertByID(r.Context(), id)
		if fetchErr == repository.ErrNotFound {
			jsonNotFound(w, "alert tidak ditemukan")
			return
		}
		if fetchErr != nil {
			jsonServerError(w, fetchErr.Error())
			return
		}
		if !claims.CanReadSiswa(alert.MstSiswaID) {
			jsonForbidden(w)
			return
		}
	}

	if err := h.svc.ResolveAlert(r.Context(), id, claims.UserID); err != nil {
		if err == repository.ErrNotFound {
			jsonNotFound(w, "alert tidak ditemukan")
			return
		}
		jsonServerError(w, err.Error())
		return
	}
	jsonOK(w, nil, "alert resolved")
}
