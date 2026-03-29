package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestHostManager_HealthCheck(t *testing.T) {
	// Start a dummy gRPC server with health check
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	port := lis.Addr().(*net.TCPAddr).Port
	grpcServer := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Logf("grpc server serve error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	hm := NewHostManager()
	hm.processes["plugin-1"] = &PluginProcess{
		ID:        "plugin-1",
		Command:   "dummy",
		Port:      port,
		StartedAt: time.Now(),
		Healthy:   false,
	}

	ctx := context.Background()

	// 1. Success case
	healthy, err := hm.HealthCheck(ctx, "plugin-1")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !healthy {
		t.Errorf("expected plugin to be healthy")
	}

	// 2. Failure case (Not serving)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	healthy, err = hm.HealthCheck(ctx, "plugin-1")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if healthy {
		t.Errorf("expected plugin to be unhealthy")
	}

	// 3. Error case (No server)
	hm.processes["plugin-error"] = &PluginProcess{
		ID:        "plugin-error",
		Command:   "dummy",
		Port:      port + 1, // Port where no server is running
		StartedAt: time.Now(),
		Healthy:   true,
	}

	healthy, err = hm.HealthCheck(ctx, "plugin-error")
	if err == nil {
		t.Errorf("expected error, got nil")
	}
	if healthy {
		t.Errorf("expected plugin to be unhealthy on error")
	}

	// Check state updated to unhealthy
	hm.mu.RLock()
	proc := hm.processes["plugin-error"]
	hm.mu.RUnlock()
	if proc.Healthy {
		t.Errorf("expected proc state to be healthy=false")
	}
}
