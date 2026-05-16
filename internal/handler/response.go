package handler

import (
	"encoding/json"
	"net/http"
)

type apiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type paginatedResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	Meta    pageMeta    `json:"meta"`
}

type pageMeta struct {
	Total   int `json:"total"`
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

func jsonOK(w http.ResponseWriter, data interface{}, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiResponse{Success: true, Message: message, Data: data})
}

func jsonPaginated(w http.ResponseWriter, data interface{}, total, page, perPage int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(paginatedResponse{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    pageMeta{Total: total, Page: page, PerPage: perPage},
	})
}

func jsonError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(apiResponse{Success: false, Message: message})
}

func jsonNotFound(w http.ResponseWriter, msg string) {
	jsonError(w, http.StatusNotFound, msg)
}

func jsonBadRequest(w http.ResponseWriter, msg string) {
	jsonError(w, http.StatusBadRequest, msg)
}

func jsonServerError(w http.ResponseWriter, msg string) {
	jsonError(w, http.StatusInternalServerError, msg)
}
