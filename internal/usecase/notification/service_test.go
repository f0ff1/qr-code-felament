package notification

import (
	"testing"
	"time"
)

func TestServicePublishAndList(t *testing.T) {
	service := NewService()

	service.Publish("filament_low", "Spool is running low", map[string]any{"spool_id": "abc"})
	items := service.List()

	if len(items) != 1 {
		t.Fatalf("len(List()) = %d, want 1", len(items))
	}
	if items[0].Type != "filament_low" {
		t.Fatalf("Type = %s, want filament_low", items[0].Type)
	}
}

func TestServiceSubscribeReceivesEvent(t *testing.T) {
	service := NewService()
	ch := service.Subscribe()

	service.Publish("printer_online", "Printer is online", nil)

	select {
	case evt := <-ch:
		if evt.Type != "printer_online" {
			t.Fatalf("event type = %s, want printer_online", evt.Type)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected notification on subscribe channel")
	}
}

func TestServicePublishDoesNotPanicWhenSubscriberClosed(t *testing.T) {
	service := NewService()
	ch := service.Subscribe()
	service.Unsubscribe(ch)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Publish panicked after unsubscribe: %v", r)
		}
	}()

	service.Publish("printer_offline", "Printer disconnected", nil)
	if got := len(service.List()); got != 1 {
		t.Fatalf("len(List()) = %d, want 1", got)
	}
}
