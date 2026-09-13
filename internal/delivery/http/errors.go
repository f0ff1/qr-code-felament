package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"filamenttracker/internal/domain"
	authusecase "filamenttracker/internal/usecase/auth"
)

type apiErrorBody struct {
	Status  string `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
	HTTP    int    `json:"http_status"`
}

func writeAPIError(w http.ResponseWriter, err error, fallbackStatus int) {
	status, code, message := mapError(err, fallbackStatus)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiErrorBody{
		Status:  "error",
		Code:    code,
		Message: message,
		HTTP:    status,
	})
}

func jsonError(w http.ResponseWriter, message string, status int) {
	writeAPIError(w, errors.New(message), status)
}

func mapError(err error, fallbackStatus int) (status int, code string, message string) {
	if err == nil {
		return fallbackStatus, "error", localizeCode("error")
	}
	switch {
	case errors.Is(err, authusecase.ErrUnauthorized):
		return http.StatusUnauthorized, "unauthorized", localizeCode("unauthorized")
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, "not_found", localizeCode("not_found")
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict, "conflict", localizeCode("conflict")
	case errors.Is(err, domain.ErrInvalid):
		return http.StatusBadRequest, "invalid_input", localizeAPIError(err.Error())
	default:
		msg := err.Error()
		return fallbackStatus, guessCode(msg, fallbackStatus), localizeAPIError(msg)
	}
}

func guessCode(message string, status int) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "unauthorized"):
		return "unauthorized"
	case strings.Contains(lower, "not found"):
		return "not_found"
	case strings.Contains(lower, "method not allowed"):
		return "method_not_allowed"
	case status == http.StatusUnauthorized:
		return "unauthorized"
	case status == http.StatusNotFound:
		return "not_found"
	case status == http.StatusMethodNotAllowed:
		return "method_not_allowed"
	default:
		return "error"
	}
}

func localizeCode(code string) string {
	switch code {
	case "unauthorized":
		return "Требуется вход в систему"
	case "not_found":
		return "Объект не найден"
	case "conflict":
		return "Конфликт данных"
	case "method_not_allowed":
		return "Метод не поддерживается"
	default:
		return "Произошла ошибка. Попробуйте ещё раз."
	}
}

func localizeAPIError(message string) string {
	msg := strings.TrimSpace(message)
	lower := strings.ToLower(msg)
	switch {
	case msg == "":
		return localizeCode("error")
	case strings.Contains(lower, "insufficient filament"):
		return "На катушке не хватает пластика для этой печати"
	case strings.Contains(lower, "invalid printer id"):
		return "Некорректный идентификатор принтера"
	case strings.Contains(lower, "invalid product id"):
		return "Некорректный идентификатор продукта"
	case strings.Contains(lower, "invalid spool id"):
		return "Некорректный идентификатор катушки"
	case strings.Contains(lower, "invalid job id"):
		return "Некорректный идентификатор задачи"
	case strings.Contains(lower, "invalid payload"):
		return "Некорректные данные запроса"
	case strings.Contains(lower, "method not allowed"):
		return localizeCode("method_not_allowed")
	case strings.Contains(lower, "unauthorized"):
		return localizeCode("unauthorized")
	case strings.Contains(lower, "not found"):
		return localizeCode("not_found")
	case strings.Contains(lower, "job is not a draft"):
		return "Эта задача уже не черновик"
	case strings.Contains(lower, "invalid material"):
		return "Некорректный материал катушки"
	case strings.Contains(lower, "invalid brand") || strings.Contains(lower, "invalid manufacturer"):
		return "Некорректный производитель катушки"
	case strings.Contains(lower, "invalid color"):
		return "Некорректный цвет катушки"
	case strings.Contains(lower, "invalid input"):
		cleaned := strings.TrimSpace(strings.TrimPrefix(msg, "invalid input:"))
		cleaned = strings.TrimSpace(strings.TrimPrefix(cleaned, "invalid input"))
		if cleaned == "" || cleaned == msg {
			return "Некорректные данные"
		}
		return localizeAPIError(cleaned)
	default:
		if strings.HasPrefix(lower, "invalid ") {
			return "Некорректные данные запроса"
		}
		return msg
	}
}
