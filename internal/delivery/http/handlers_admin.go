package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"filamenttracker/internal/bootstrap"
	userdomain "filamenttracker/internal/domain/user"
	adminusecase "filamenttracker/internal/usecase/admin"
	authusecase "filamenttracker/internal/usecase/auth"

	"github.com/google/uuid"
)

func registerAdminRoutes(mux *http.ServeMux, app *bootstrap.App) {
	mux.HandleFunc("/api/admin/sites", func(w http.ResponseWriter, r *http.Request) {
		sess, ok := sessionFrom(r.Context())
		if !ok || !requireAdmin(sess) {
			writeAPIError(w, authusecase.ErrUnauthorized, http.StatusForbidden)
			return
		}
		switch r.Method {
		case http.MethodGet:
			sites, err := app.AdminService.ListSites(r.Context())
			if err != nil {
				writeAPIError(w, err, http.StatusInternalServerError)
				return
			}
			payload := make([]map[string]any, 0, len(sites))
			for _, s := range sites {
				payload = append(payload, map[string]any{
					"id":              s.ID.String(),
					"organization_id": s.OrganizationID.String(),
					"name":            s.Name,
					"address":         s.Address,
				})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		case http.MethodPost:
			var input struct {
				OrganizationName string `json:"organization_name"`
				Name             string `json:"name"`
				Address          string `json:"address"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				jsonError(w, "invalid payload", http.StatusBadRequest)
				return
			}
			site, err := app.AdminService.CreateSite(r.Context(), adminusecase.CreateSiteInput{
				OrganizationName: input.OrganizationName,
				Name:             input.Name,
				Address:          input.Address,
			})
			if err != nil {
				writeAPIError(w, err, http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":              site.ID.String(),
				"organization_id": site.OrganizationID.String(),
				"name":            site.Name,
				"address":         site.Address,
			})
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/admin/users", func(w http.ResponseWriter, r *http.Request) {
		sess, ok := sessionFrom(r.Context())
		if !ok || !requireAdmin(sess) {
			writeAPIError(w, authusecase.ErrUnauthorized, http.StatusForbidden)
			return
		}
		switch r.Method {
		case http.MethodGet:
			users, err := app.AdminService.ListUsers(r.Context())
			if err != nil {
				writeAPIError(w, err, http.StatusInternalServerError)
				return
			}
			payload := make([]map[string]any, 0, len(users))
			for _, u := range users {
				payload = append(payload, userJSON(u))
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		case http.MethodPost:
			var input struct {
				Username  string   `json:"username"`
				Password  string   `json:"password"`
				FirstName string   `json:"first_name"`
				LastName  string   `json:"last_name"`
				Role      string   `json:"role"`
				SiteIDs   []string `json:"site_ids"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				jsonError(w, "invalid payload", http.StatusBadRequest)
				return
			}
			siteIDs := make([]uuid.UUID, 0, len(input.SiteIDs))
			for _, raw := range input.SiteIDs {
				id, err := uuid.Parse(strings.TrimSpace(raw))
				if err != nil {
					jsonError(w, "invalid site id", http.StatusBadRequest)
					return
				}
				siteIDs = append(siteIDs, id)
			}
			result, err := app.AdminService.CreateUser(r.Context(), adminusecase.CreateUserInput{
				Username:  input.Username,
				Password:  input.Password,
				FirstName: input.FirstName,
				LastName:  input.LastName,
				Role:      userdomain.Role(input.Role),
				SiteIDs:   siteIDs,
			})
			if err != nil {
				writeAPIError(w, err, http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user":     userJSON(result.User),
				"password": result.PlaintextPassword,
			})
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/admin/users/", func(w http.ResponseWriter, r *http.Request) {
		sess, ok := sessionFrom(r.Context())
		if !ok || !requireAdmin(sess) {
			writeAPIError(w, authusecase.ErrUnauthorized, http.StatusForbidden)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			jsonError(w, "missing user id", http.StatusBadRequest)
			return
		}
		userID, err := uuid.Parse(parts[0])
		if err != nil {
			jsonError(w, "invalid user id", http.StatusBadRequest)
			return
		}
		if len(parts) == 2 && parts[1] == "password" && r.Method == http.MethodPost {
			var input struct {
				Password string `json:"password"`
			}
			_ = json.NewDecoder(r.Body).Decode(&input)
			plain, err := app.AdminService.SetUserPassword(r.Context(), userID, input.Password)
			if err != nil {
				writeAPIError(w, err, http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"password": plain})
			return
		}
		if len(parts) == 2 && parts[1] == "active" && r.Method == http.MethodPost {
			var input struct {
				Active bool `json:"active"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				jsonError(w, "invalid payload", http.StatusBadRequest)
				return
			}
			if err := app.AdminService.SetUserActive(r.Context(), userID, input.Active); err != nil {
				writeAPIError(w, err, http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		jsonError(w, "not found", http.StatusNotFound)
	})

	mux.HandleFunc("/api/admin/password-resets", func(w http.ResponseWriter, r *http.Request) {
		sess, ok := sessionFrom(r.Context())
		if !ok || !requireAdmin(sess) {
			writeAPIError(w, authusecase.ErrUnauthorized, http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		reqs, err := app.AdminService.ListOpenResetRequests(r.Context())
		if err != nil {
			writeAPIError(w, err, http.StatusInternalServerError)
			return
		}
		payload := make([]map[string]any, 0, len(reqs))
		for _, req := range reqs {
			payload = append(payload, map[string]any{
				"id":         req.ID.String(),
				"user_id":    req.UserID.String(),
				"status":     req.Status,
				"created_at": req.CreatedAt,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	})

	mux.HandleFunc("/api/admin/password-resets/", func(w http.ResponseWriter, r *http.Request) {
		sess, ok := sessionFrom(r.Context())
		if !ok || !requireAdmin(sess) {
			writeAPIError(w, authusecase.ErrUnauthorized, http.StatusForbidden)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/admin/password-resets/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) != 2 || parts[1] != "resolve" || r.Method != http.MethodPost {
			jsonError(w, "not found", http.StatusNotFound)
			return
		}
		reqID, err := uuid.Parse(parts[0])
		if err != nil {
			jsonError(w, "invalid request id", http.StatusBadRequest)
			return
		}
		var input struct {
			Password string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&input)
		plain, err := app.AdminService.ResolveResetAndSetPassword(r.Context(), reqID, input.Password)
		if err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"password": plain})
	})
}

func userJSON(u userdomain.User) map[string]any {
	return map[string]any{
		"id":         u.ID.String(),
		"username":   u.Username,
		"first_name": u.FirstName,
		"last_name":  u.LastName,
		"role":       string(u.Role),
		"is_active":  u.IsActive,
		"site_ids":   uuidStrings(u.SiteIDs),
	}
}
