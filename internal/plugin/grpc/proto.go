package grpc

// -- gRPC Service Definitions (plain Go, proto-compatible) ------
//
// These types mirror the future .proto definitions.
// When we add protobuf/gRPC dependencies, these will be replaced
// by generated code from plugin_service.proto.

// PluginInfoResponse mirrors the proto PluginInfo message.
type PluginInfoResponse struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Type       string   `json:"type"`
	TrustLevel string   `json:"trust_level"`
	Hooks      []string `json:"hooks"`
}

// HealthCheckResponse mirrors the proto HealthCheck response.
type HealthCheckResponse struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message"`
}

// HookEventMessage mirrors the proto HookEvent message.
type HookEventMessage struct {
	Hook    string            `json:"hook"`
	Payload map[string]string `json:"payload"` // Simplified to string values for proto compat
}

// SkillMatchRequest mirrors the proto SkillMatch request.
type SkillMatchRequest struct {
	RawText     string  `json:"raw_text"`
	Category    string  `json:"category"`
	Confidence  float64 `json:"confidence"`
	KnowledgeID uint64  `json:"knowledge_id"`
}

// SkillMatchResponse mirrors the proto SkillMatch response.
type SkillMatchResponse struct {
	Confidence float64 `json:"confidence"`
}
