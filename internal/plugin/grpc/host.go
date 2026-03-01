// Package grpc implements process isolation for community plugins.
// Community-trust plugins run in separate processes and communicate
// with the host via gRPC, providing security sandboxing.
//
// Reference: design.md §7.6 — Runtime Isolation
package grpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hflms/hanfledge/internal/infrastructure/logger"
)

var slogGRPC = logger.L("PluginGRPC")

// PluginProcess represents a running plugin in a separate process.
type PluginProcess struct {
	ID        string
	Command   string // Path to plugin binary
	Port      int
	StartedAt time.Time
	Healthy   bool
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

	proc := &PluginProcess{
		ID:        pluginID,
		Command:   binaryPath,
		Port:      port,
		StartedAt: time.Now(),
		Healthy:   false,
	}

	// TODO: Actually launch the subprocess with exec.Command
	// and establish gRPC connection
	slogGRPC.Info("starting plugin", "plugin", pluginID, "port", port, "binary", binaryPath)

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

	// TODO: Send graceful shutdown signal, then kill after timeout
	slogGRPC.Info("stopping plugin", "plugin", pluginID, "port", proc.Port)

	delete(h.processes, pluginID)
	return nil
}

// HealthCheck checks if a plugin process is healthy.
func (h *HostManager) HealthCheck(ctx context.Context, pluginID string) (bool, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	proc, exists := h.processes[pluginID]
	if !exists {
		return false, fmt.Errorf("plugin %s is not running", pluginID)
	}

	// TODO: Call gRPC HealthCheck method
	return proc.Healthy, nil
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
