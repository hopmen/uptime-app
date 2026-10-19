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

type createMonitorRequest struct {
	// URL сайта с протоколом http:// или https://, без логина и пароля.
	URL             string `json:"url" validate:"required" example:"https://example.com"`
	IntervalSeconds int64  `json:"intervalSeconds" validate:"required" minimum:"1" maximum:"2147483647" example:"60"`
}

// create godoc
// @Summary Создать точку мониторинга
// @Description Сохраняет URL и интервал в секундах; проверки сайта пока не запускаются.
// @ID createMonitor
// @Tags monitors
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createMonitorRequest true "Настройки точки мониторинга"
// @Success 201 {object} monitor.Monitor
// @Failure 400 {object} errorResponse "Некорректный JSON, URL или интервал"
// @Failure 401 {object} errorResponse "Неверный access token"
// @Failure 500 {object} errorResponse "Не удалось сохранить точку мониторинга"
// @Router /api/v1/monitors [post]
func (h *monitorHandler) create(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.authenticatedUser(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var request createMonitorRequest
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

// list godoc
// @Summary Получить точки мониторинга текущего пользователя
// @Description Возвращает массив, в том числе пустой, без пагинации и параметров фильтрации.
// @ID listMonitors
// @Tags monitors
// @Produce json
// @Security BearerAuth
// @Success 200 {array} monitor.Monitor
// @Failure 401 {object} errorResponse "Неверный access token"
// @Failure 500 {object} errorResponse "Не удалось загрузить точки мониторинга"
// @Router /api/v1/monitors [get]
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
