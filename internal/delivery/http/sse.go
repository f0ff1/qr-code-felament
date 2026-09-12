package http

import (
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
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		ch := service.Subscribe()
		defer service.Unsubscribe(ch)

		for {
			select {
			case <-r.Context().Done():
				return
			case evt := <-ch:
				_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, toJSON(evt))
				flusher.Flush()
			case <-time.After(15 * time.Second):
				_, _ = fmt.Fprint(w, ": heartbeat\n\n")
				flusher.Flush()
			}
		}
	}
}

func toJSON(evt notificationusecase.Event) string {
	return fmt.Sprintf("{\"id\":\"%s\",\"type\":\"%s\",\"message\":\"%s\",\"created_at\":\"%s\"}", evt.ID, evt.Type, evt.Message, evt.CreatedAt.Format(time.RFC3339))
}
