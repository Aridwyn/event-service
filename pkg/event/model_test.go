package event

import (
	"encoding/json"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestState_String(t *testing.T) {
	tests := []struct {
		name     string
		state    State
		expected string
	}{
		{"Active", Active, "started"},
		{"Finished", Finished, "finished"},
		{"Unknown", State(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.state.String()
			if result != tt.expected {
				t.Errorf("State.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestEvent_ToResponse_WithFinishedAt(t *testing.T) {
	finishedAt := time.Now()
	event := &Event{
		ID:         primitive.NewObjectID(),
		Type:       "meeting",
		State:      Finished,
		StartedAt:  time.Now().Add(-1 * time.Hour),
		FinishedAt: &finishedAt,
	}

	resp := event.ToResponse()

	if resp.ID == "" {
		t.Error("ID should not be empty")
	}
	if resp.ID == "000000000000000000000000" {
		t.Error("ID should not be zero value")
	}
	if resp.Type != "meeting" {
		t.Errorf("Expected type 'meeting', got %s", resp.Type)
	}
	if resp.State != "finished" {
		t.Errorf("Expected state 'finished', got %s", resp.State)
	}
	if resp.FinishedAt == nil {
		t.Error("FinishedAt should not be nil")
	}
	if !resp.FinishedAt.Equal(finishedAt) {
		t.Error("FinishedAt should match")
	}
}

func TestEvent_ToResponse_WithoutFinishedAt(t *testing.T) {
	event := &Event{
		ID:         primitive.NewObjectID(),
		Type:       "meeting",
		State:      Active,
		StartedAt:  time.Now(),
		FinishedAt: nil,
	}

	resp := event.ToResponse()

	if resp.ID == "" {
		t.Error("ID should not be empty")
	}
	if resp.ID == "000000000000000000000000" {
		t.Error("ID should not be zero value")
	}
	if resp.Type != "meeting" {
		t.Errorf("Expected type 'meeting', got %s", resp.Type)
	}
	if resp.State != "started" {
		t.Errorf("Expected state 'started', got %s", resp.State)
	}
	if resp.FinishedAt != nil {
		t.Error("FinishedAt should be nil for active event")
	}
}

func TestEvent_MarshalJSON(t *testing.T) {
	finishedAt := time.Now()
	event := &Event{
		ID:         primitive.NewObjectID(),
		Type:       "meeting",
		State:      Finished,
		StartedAt:  time.Now().Add(-1 * time.Hour),
		FinishedAt: &finishedAt,
	}

	data, err := event.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var resp EventResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	if resp.Type != "meeting" {
		t.Errorf("Expected type 'meeting', got %s", resp.Type)
	}
	if resp.State != "finished" {
		t.Errorf("Expected state 'finished', got %s", resp.State)
	}
	if resp.FinishedAt == nil {
		t.Error("FinishedAt should not be nil")
	}
}

func TestEvent_MarshalJSON_ActiveEvent(t *testing.T) {
	event := &Event{
		ID:         primitive.NewObjectID(),
		Type:       "call",
		State:      Active,
		StartedAt:  time.Now(),
		FinishedAt: nil,
	}

	data, err := event.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var resp EventResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	if resp.Type != "call" {
		t.Errorf("Expected type 'call', got %s", resp.Type)
	}
	if resp.State != "started" {
		t.Errorf("Expected state 'started', got %s", resp.State)
	}
	if resp.FinishedAt != nil {
		t.Error("FinishedAt should be nil for active event")
	}
}
