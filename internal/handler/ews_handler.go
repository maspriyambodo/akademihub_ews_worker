package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sekolahpintar/ews-worker/internal/repository"
	"github.com/sekolahpintar/ews-worker/internal/service"
)

type EWSHandler struct {
	svc *service.EWSService
}

func NewEWSHandler(svc *service.EWSService) *EWSHandler {
	return &EWSHandler{svc: svc}
}

// Process POST /api/v1/ews/process — trigger full batch EWS processing.
func (h *EWSHandler) Process(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.ProcessAll(r.Context())
	if err != nil {
		jsonServerError(w, err.Error())
		return
	}
	jsonOK(w, result, "EWS batch processing complete")
}

// ProcessSiswa POST /api/v1/ews/process-siswa/{siswaId} — process a single student.
func (h *EWSHandler) ProcessSiswa(w http.ResponseWriter, r *http.Request) {
	siswaID, err := strconv.ParseInt(chi.URLParam(r, "siswaId"), 10, 64)
	if err != nil {
		jsonBadRequest(w, "invalid siswaId")
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
func (h *EWSHandler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filters := map[string]string{
		"mst_siswa_id": q.Get("mst_siswa_id"),
		"is_resolved":  q.Get("is_resolved"),
		"level":        q.Get("level"),
		"kategori":     q.Get("kategori"),
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
func (h *EWSHandler) GetAlertsBySiswa(w http.ResponseWriter, r *http.Request) {
	siswaID, err := strconv.ParseInt(chi.URLParam(r, "siswaId"), 10, 64)
	if err != nil {
		jsonBadRequest(w, "invalid siswaId")
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
func (h *EWSHandler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonBadRequest(w, "invalid id")
		return
	}

	if err := h.svc.ResolveAlert(r.Context(), id); err != nil {
		if err == repository.ErrNotFound {
			jsonNotFound(w, "alert tidak ditemukan")
			return
		}
		jsonServerError(w, err.Error())
		return
	}
	jsonOK(w, nil, "alert resolved")
}
