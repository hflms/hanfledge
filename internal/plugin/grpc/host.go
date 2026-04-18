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
	"google.golang.org/grpc/credentials/local"
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
	waitCh    chan struct{}
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

	if _, exists := h.processes[pluginID]; exists {
		h.mu.Unlock()
		return nil, fmt.Errorf("plugin %s is already running", pluginID)
	}

	port := h.nextPort
	h.nextPort++

	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))

	if err := cmd.Start(); err != nil {
		h.mu.Unlock()
		return nil, fmt.Errorf("failed to start plugin process: %w", err)
	}

	proc := &PluginProcess{
		ID:        pluginID,
		Command:   binaryPath,
		Port:      port,
		StartedAt: time.Now(),
		Healthy:   false,
		cmd:       cmd,
		waitCh:    make(chan struct{}),
	}

	slogGRPC.Info("starting plugin", "plugin", pluginID, "port", port, "binary", binaryPath)

	h.processes[pluginID] = proc
	h.mu.Unlock()

	go func() {
		err := cmd.Wait()
		if err != nil {
			slogGRPC.Error("plugin process exited with error", "plugin", pluginID, "error", err)
		} else {
			slogGRPC.Info("plugin process exited normally", "plugin", pluginID)
		}
		h.updateHealth(pluginID, false)
		close(proc.waitCh)
	}()

	// Wait for process to be healthy
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	isHealthy := false
	for {
		select {
		case <-ctxTimeout.Done():
			goto DoneWaiting
		case <-proc.waitCh:
			goto DoneWaiting
		default:
			// We no longer hold h.mu, so we can safely call HealthCheck
			healthy, err := h.HealthCheck(ctxTimeout, pluginID)
			if err == nil && healthy {
				isHealthy = true
				goto DoneWaiting
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

DoneWaiting:
	if !isHealthy {
		// Clean up on failure
		if proc.cmd.Process != nil {
			proc.cmd.Process.Kill()
		}
		h.mu.Lock()
		delete(h.processes, pluginID)
		h.mu.Unlock()
		return nil, fmt.Errorf("plugin %s failed to become healthy within timeout or exited prematurely", pluginID)
	}

	h.updateHealth(pluginID, true)

	// Background health check monitor
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-proc.waitCh:
				return // Process exited, stop monitoring
			case <-ticker.C:
				// Background monitor should use a short timeout and update state
				ctxCheck, cancelCheck := context.WithTimeout(context.Background(), 2*time.Second)
				healthy, err := h.HealthCheck(ctxCheck, pluginID)
				cancelCheck()

				if err != nil || !healthy {
					h.updateHealth(pluginID, false)
				} else {
					h.updateHealth(pluginID, true)
				}
			}
		}
	}()

	return proc, nil
}

// Stop terminates a running plugin process.
func (h *HostManager) Stop(ctx context.Context, pluginID string) error {
	h.mu.Lock()

	proc, exists := h.processes[pluginID]
	if !exists {
		h.mu.Unlock()
		return fmt.Errorf("plugin %s is not running", pluginID)
	}

	slogGRPC.Info("stopping plugin", "plugin", pluginID, "port", proc.Port)

	// We release the lock immediately to not block other operations while waiting for shutdown.
	// We'll reacquire it at the end to delete the process from the map.
	h.mu.Unlock()

	if proc.cmd != nil && proc.cmd.Process != nil {
		err := proc.cmd.Process.Signal(os.Interrupt)
		if err != nil {
			slogGRPC.Warn("failed to send interrupt signal, killing immediately", "plugin", pluginID, "error", err)
			proc.cmd.Process.Kill()
		} else {
			// Wait for process to exit or timeout
			select {
			case <-proc.waitCh:
				slogGRPC.Info("plugin process exited gracefully", "plugin", pluginID)
			case <-time.After(5 * time.Second):
				slogGRPC.Warn("plugin process did not exit gracefully, killing", "plugin", pluginID)
				proc.cmd.Process.Kill()
				// Wait for waitCh to ensure resources are reclaimed, but with a short timeout just in case
				select {
				case <-proc.waitCh:
				case <-time.After(1 * time.Second):
				}
			}
		}
	}

	h.mu.Lock()
	delete(h.processes, pluginID)
	h.mu.Unlock()

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
	conn, err := googlegrpc.NewClient(addr, googlegrpc.WithTransportCredentials(local.NewCredentials()))
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
