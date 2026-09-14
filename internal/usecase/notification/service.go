package notification

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

const DefaultTTL = 24 * time.Hour

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
	ttl   time.Duration
}

func NewService() *Service {
	return NewServiceWithTTL(DefaultTTL)
}

func NewServiceWithTTL(ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Service{subs: make(map[chan Event]struct{}), ttl: ttl}
}

func (s *Service) Publish(eventType, message string, payload map[string]any) Event {
	event := Event{
		ID:        uuid.NewString(),
		Type:      eventType,
		Message:   message,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	s.pruneLocked(time.Now().UTC())
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(time.Now().UTC())
	out := make([]Event, len(s.items))
	copy(out, s.items)
	return out
}

func (s *Service) ListRecent(limit int) []Event {
	all := s.List()
	if limit <= 0 || len(all) <= limit {
		return all
	}
	return all[len(all)-limit:]
}

func (s *Service) pruneLocked(now time.Time) {
	ttl := s.ttl
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	cutoff := now.Add(-ttl)
	kept := s.items[:0]
	for _, item := range s.items {
		if item.CreatedAt.After(cutoff) {
			kept = append(kept, item)
		}
	}
	s.items = kept
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
