package lms

import (
	"context"
	"testing"
)

func TestNewSCORMAdapter(t *testing.T) {
	tests := []struct {
		name    string
		cfg     LMSConfig
		wantErr bool
	}{
		{
			name: "valid configuration",
			cfg: LMSConfig{
				SCORMEndpoint: "http://example.com/scorm",
				SCORMAPIKey:   "secret-key",
			},
			wantErr: false,
		},
		{
			name: "missing endpoint",
			cfg: LMSConfig{
				SCORMAPIKey: "secret-key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewSCORMAdapter(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSCORMAdapter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && adapter == nil {
				t.Errorf("NewSCORMAdapter() returned nil adapter when expecting valid one")
			}
		})
	}
}

func TestSCORMAdapter_Type(t *testing.T) {
	adapter := &SCORMAdapter{}
	if got := adapter.Type(); got != AdapterSCORM {
		t.Errorf("SCORMAdapter.Type() = %v, want %v", got, AdapterSCORM)
	}
}

func TestSCORMAdapter_LaunchURL(t *testing.T) {
	adapter := &SCORMAdapter{
		endpoint: "http://example.com/scorm",
		apiKey:   "test-key",
	}

	req := LaunchRequest{
		UserID:     "user1",
		CourseID:   "course1",
		ActivityID: "activity1",
	}

	resp, err := adapter.LaunchURL(context.Background(), req)
	if err != nil {
		t.Fatalf("LaunchURL() unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatalf("LaunchURL() returned nil response")
	}

	if resp.URL != "http://example.com/scorm" {
		t.Errorf("LaunchURL() URL = %v, want %v", resp.URL, "http://example.com/scorm")
	}

	if resp.Method != "POST" {
		t.Errorf("LaunchURL() Method = %v, want %v", resp.Method, "POST")
	}

	expectedSessionID := "scorm_user1_activity1"
	if resp.SessionID != expectedSessionID {
		t.Errorf("LaunchURL() SessionID = %v, want %v", resp.SessionID, expectedSessionID)
	}

	if resp.FormData["user_id"] != "user1" {
		t.Errorf("LaunchURL() FormData[user_id] = %v, want %v", resp.FormData["user_id"], "user1")
	}

	if resp.FormData["api_key"] != "test-key" {
		t.Errorf("LaunchURL() FormData[api_key] = %v, want %v", resp.FormData["api_key"], "test-key")
	}
}

func TestSCORMAdapter_Validate(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		{
			name:     "valid endpoint",
			endpoint: "http://example.com/scorm",
			wantErr:  false,
		},
		{
			name:     "empty endpoint",
			endpoint: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &SCORMAdapter{endpoint: tt.endpoint}
			err := adapter.Validate(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
