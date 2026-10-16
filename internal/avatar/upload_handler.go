package avatar

import (
	"encoding/json"
	"errors"
	"net/http"
)

type UploadHandler struct {
	pipeline *Pipeline
}

func NewUploadHandler(pipeline *Pipeline) *UploadHandler {
	return &UploadHandler{pipeline: pipeline}
}

func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/avatars" {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart request"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
		return
	}
	defer file.Close()

	input := UploadInput{
		UserID: r.FormValue("user_id"), Filename: header.Filename, Content: file,
		ProfileKind: ProfileKind(r.FormValue("profile_kind")), RequestID: r.Header.Get("Idempotency-Key"),
	}
	if input.UserID == "" || input.RequestID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id and Idempotency-Key are required"})
		return
	}

	result, err := h.pipeline.Process(r.Context(), input)
	if err != nil {
		status := http.StatusBadGateway
		var infraiErr *InfraiError
		var inputErr *InvalidInputError
		if errors.As(err, &inputErr) {
			status = http.StatusBadRequest
		} else if errors.As(err, &infraiErr) && infraiErr.HTTPStatus >= 400 && infraiErr.HTTPStatus < 500 {
			status = infraiErr.HTTPStatus
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
