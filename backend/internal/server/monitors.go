package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"uptime-app/backend/internal/monitor"
)

type monitorHandler struct {
	auth    *authHandler
	service *monitor.Service
}

func (h *monitorHandler) create(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.authenticatedUser(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var request struct {
		URL             string `json:"url"`
		IntervalSeconds int64  `json:"intervalSeconds"`
	}
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "некорректное тело запроса")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "ожидается один JSON-объект")
		return
	}
	point, err := h.service.Create(r.Context(), user.ID, request.URL, request.IntervalSeconds)
	if err != nil {
		if errors.Is(err, monitor.ErrInvalidURL) || errors.Is(err, monitor.ErrInvalidInterval) {
			writeError(w, http.StatusBadRequest, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "не удалось сохранить точку мониторинга")
		}
		return
	}
	writeJSON(w, http.StatusCreated, point)
}

func (h *monitorHandler) list(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.authenticatedUser(w, r)
	if !ok {
		return
	}
	points, err := h.service.List(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось загрузить точки мониторинга")
		return
	}
	writeJSON(w, http.StatusOK, points)
}
