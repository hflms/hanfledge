// Package grpc implements process isolation for community plugins.
// Community-trust plugins run in separate processes and communicate
// with the host via gRPC, providing security sandboxing.
//
// Reference: design.md §7.6 — Runtime Isolation
package grpc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var slogGRPC = logger.L("PluginGRPC")

// PluginProcess represents a running plugin in a separate process.
type PluginProcess struct {
	ID        string
	Command   string // Path to plugin binary
	Port      int
	StartedAt time.Time
	Healthy   bool
	cmd       *exec.Cmd
}

// HostManager manages gRPC plugin processes on the host side.
type HostManager struct {
	mu        sync.RWMutex
	processes map[string]*PluginProcess
	nextPort  int
}

// NewHostManager creates a new gRPC plugin host manager.
func NewHostManager() *HostManager {
	return &HostManager{
		processes: make(map[string]*PluginProcess),
		nextPort:  50100, // gRPC plugins start at port 50100
	}
}

// Start launches a community plugin in a separate process.
// The plugin binary must implement the PluginService gRPC interface.
func (h *HostManager) Start(ctx context.Context, pluginID string, binaryPath string) (*PluginProcess, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.processes[pluginID]; exists {
		return nil, fmt.Errorf("plugin %s is already running", pluginID)
	}

	port := h.nextPort
	h.nextPort++

	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start plugin process: %w", err)
	}

	proc := &PluginProcess{
		ID:        pluginID,
		Command:   binaryPath,
		Port:      port,
		StartedAt: time.Now(),
		Healthy:   false,
		cmd:       cmd,
	}

	slogGRPC.Info("starting plugin", "plugin", pluginID, "port", port, "binary", binaryPath)

	go func() {
		err := cmd.Wait()
		if err != nil {
			slogGRPC.Error("plugin process exited with error", "plugin", pluginID, "error", err)
		} else {
			slogGRPC.Info("plugin process exited normally", "plugin", pluginID)
		}
		h.updateHealth(pluginID, false)
	}()

	proc.Healthy = true
	h.processes[pluginID] = proc

	return proc, nil
}

// Stop terminates a running plugin process.
func (h *HostManager) Stop(ctx context.Context, pluginID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	proc, exists := h.processes[pluginID]
	if !exists {
		return fmt.Errorf("plugin %s is not running", pluginID)
	}

	slogGRPC.Info("stopping plugin", "plugin", pluginID, "port", proc.Port)

	if proc.cmd != nil && proc.cmd.Process != nil {
		err := proc.cmd.Process.Signal(os.Interrupt)
		if err != nil {
			slogGRPC.Warn("failed to send interrupt signal, killing immediately", "plugin", pluginID, "error", err)
			proc.cmd.Process.Kill()
		} else {
			// Instead of Wait, since we are already waiting in a goroutine in Start(),
			// we can't call Wait() again.
			// We can use a simpler approach: check if process is alive periodically
			// or just use time.AfterFunc to kill it. Wait() will clean up anyway.
			timer := time.AfterFunc(5*time.Second, func() {
				slogGRPC.Warn("plugin process did not exit gracefully, killing", "plugin", pluginID)
				proc.cmd.Process.Kill()
			})

			// If we wanted to wait synchronously in Stop, we'd need a channel in proc.
			// The original code was fine with just deleting it from map and returning.
			// time.AfterFunc is sufficient to ensure it dies.
			_ = timer
		}
	}

	delete(h.processes, pluginID)
	return nil
}

// HealthCheck checks if a plugin process is healthy.
func (h *HostManager) HealthCheck(ctx context.Context, pluginID string) (bool, error) {
	h.mu.RLock()
	proc, exists := h.processes[pluginID]
	if !exists {
		h.mu.RUnlock()
		return false, fmt.Errorf("plugin %s is not running", pluginID)
	}
	port := proc.Port
	h.mu.RUnlock()

	addr := fmt.Sprintf("localhost:%d", port)
	conn, err := googlegrpc.NewClient(addr, googlegrpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		h.updateHealth(pluginID, false)
		return false, fmt.Errorf("failed to connect to plugin %s: %w", pluginID, err)
	}
	defer conn.Close()

	client := grpc_health_v1.NewHealthClient(conn)
	req := &grpc_health_v1.HealthCheckRequest{Service: ""}

	ctxTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := client.Check(ctxTimeout, req)
	if err != nil {
		h.updateHealth(pluginID, false)
		return false, fmt.Errorf("health check failed for plugin %s: %w", pluginID, err)
	}

	healthy := resp.Status == grpc_health_v1.HealthCheckResponse_SERVING
	h.updateHealth(pluginID, healthy)

	return healthy, nil
}

func (h *HostManager) updateHealth(pluginID string, healthy bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if proc, exists := h.processes[pluginID]; exists {
		proc.Healthy = healthy
	}
}

// ListProcesses returns all running plugin processes.
func (h *HostManager) ListProcesses() []*PluginProcess {
	h.mu.RLock()
	defer h.mu.RUnlock()

	procs := make([]*PluginProcess, 0, len(h.processes))
	for _, p := range h.processes {
		procs = append(procs, p)
	}
	return procs
}
