package http

import (
	"net/http"

	userdomain "filamenttracker/internal/domain/user"
	authusecase "filamenttracker/internal/usecase/auth"

	"github.com/google/uuid"
)

func sameSite(entitySite, activeSite uuid.UUID) bool {
	if activeSite == uuid.Nil {
		return true
	}
	if entitySite == uuid.Nil {
		return false
	}
	return entitySite == activeSite
}

func requireWritable(w http.ResponseWriter, r *http.Request) bool {
	sess, ok := sessionFrom(r.Context())
	if !ok {
		writeAPIError(w, authusecase.ErrUnauthorized, http.StatusUnauthorized)
		return false
	}
	if sess.Role == string(userdomain.RoleViewer) {
		jsonError(w, "недостаточно прав: режим только просмотр", http.StatusForbidden)
		return false
	}
	return true
}

func eventMatchesSite(payload map[string]any, siteID uuid.UUID) bool {
	if siteID == uuid.Nil {
		return true
	}
	if payload == nil {
		return false
	}
	raw, ok := payload["site_id"]
	if !ok || raw == nil {
		return false
	}
	switch v := raw.(type) {
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			return false
		}
		return id == siteID
	default:
		return false
	}
}

func withSitePayload(payload map[string]any, siteID uuid.UUID) map[string]any {
	if payload == nil {
		payload = map[string]any{}
	}
	if siteID != uuid.Nil {
		payload["site_id"] = siteID.String()
	}
	return payload
}
