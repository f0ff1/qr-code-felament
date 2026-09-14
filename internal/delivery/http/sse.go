package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	notificationusecase "filamenttracker/internal/usecase/notification"
)

func NewSSEHandler(service *notificationusecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"status":"error","code":500,"message":"streaming unsupported"}`)
			return
		}

		siteID := activeSiteID(r.Context())
		ch := service.Subscribe()
		defer service.Unsubscribe(ch)

		for {
			select {
			case <-r.Context().Done():
				return
			case evt := <-ch:
				if !eventMatchesSite(evt.Payload, siteID) {
					continue
				}
				_, _ = fmt.Fprintf(w, "data: %s\n\n", toJSON(evt))
				flusher.Flush()
			case <-time.After(15 * time.Second):
				_, _ = fmt.Fprint(w, ": heartbeat\n\n")
				flusher.Flush()
			}
		}
	}
}

func toJSON(evt notificationusecase.Event) string {
	payload, _ := json.Marshal(evt.Payload)
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	msg, _ := json.Marshal(evt.Message)
	return fmt.Sprintf(
		`{"id":%q,"type":%q,"message":%s,"created_at":%q,"payload":%s}`,
		evt.ID,
		evt.Type,
		string(msg),
		evt.CreatedAt.Format(time.RFC3339),
		string(payload),
	)
}
