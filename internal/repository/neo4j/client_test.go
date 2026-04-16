package neo4j

import (
	"strings"
	"testing"

	"github.com/hflms/hanfledge/internal/config"
)

func TestNewClient_InvalidURI(t *testing.T) {
	cfg := &config.Neo4jConfig{
		URI:      "invalid://localhost",
		User:     "u",
		Password: "p",
	}

	client, err := NewClient(cfg)
	if err == nil {
		t.Fatal("expected error for invalid URI, got nil")
	}
	if client != nil {
		t.Fatal("expected nil client for invalid URI")
	}

	if !strings.Contains(err.Error(), "failed to create Neo4j driver") {
		t.Errorf("expected error to contain 'failed to create Neo4j driver', got: %v", err)
	}
}

func TestNewClient_ConnectivityCheckFailed(t *testing.T) {
	// Use an invalid port to trigger a connectivity failure
	cfg := &config.Neo4jConfig{
		URI:      "bolt://127.0.0.1:0", // Invalid port
		User:     "u",
		Password: "p",
	}

	client, err := NewClient(cfg)
	if err == nil {
		t.Fatal("expected error for connectivity check failure, got nil")
	}
	if client != nil {
		t.Fatal("expected nil client for connectivity check failure")
	}

	if !strings.Contains(err.Error(), "Neo4j connectivity check failed") {
		t.Errorf("expected error to contain 'Neo4j connectivity check failed', got: %v", err)
	}
}
