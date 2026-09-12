package notification

import (
	"sync"
	"time"
)

type Event struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Message   string         `json:"message"`
	Payload   map[string]any `json:"payload,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type Service struct {
	mu    sync.RWMutex
	items []Event
	subs  map[chan Event]struct{}
}

func NewService() *Service {
	return &Service{subs: make(map[chan Event]struct{})}
}

func (s *Service) Publish(eventType, message string, payload map[string]any) Event {
	event := Event{
		ID:        time.Now().UTC().Format(time.RFC3339Nano),
		Type:      eventType,
		Message:   message,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	s.items = append(s.items, event)
	for ch := range s.subs {
		func() {
			defer func() {
				if recover() != nil {
					delete(s.subs, ch)
				}
			}()
			select {
			case ch <- event:
			default:
			}
		}()
	}
	s.mu.Unlock()

	return event
}

func (s *Service) List() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Event, len(s.items))
	copy(out, s.items)
	return out
}

func (s *Service) Subscribe() chan Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan Event, 16)
	s.subs[ch] = struct{}{}
	return ch
}

func (s *Service) Unsubscribe(ch chan Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs, ch)
	func() {
		defer func() { recover() }()
		close(ch)
	}()
}
