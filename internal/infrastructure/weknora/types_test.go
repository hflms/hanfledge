package weknora

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestKnowledgeBaseJSON(t *testing.T) {
	now := time.Now().Truncate(time.Second) // JSON marshaling trims to second/millisecond based on layout
	kb := KnowledgeBase{
		ID:          "kb-1",
		Name:        "Test KB",
		Description: "A test KB",
		FileCount:   10,
		TokenCount:  1000,
		ChunkCount:  50,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	b, err := json.Marshal(kb)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var kb2 KnowledgeBase
	if err := json.Unmarshal(b, &kb2); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Compare time specially because of unmarshal precision differences
	if !kb2.CreatedAt.Equal(kb.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", kb2.CreatedAt, kb.CreatedAt)
	}
	if !kb2.UpdatedAt.Equal(kb.UpdatedAt) {
		t.Errorf("UpdatedAt mismatch: got %v, want %v", kb2.UpdatedAt, kb.UpdatedAt)
	}

	// Check the rest
	kb2.CreatedAt = kb.CreatedAt
	kb2.UpdatedAt = kb.UpdatedAt
	if !reflect.DeepEqual(kb, kb2) {
		t.Errorf("Mismatch:\ngot  %+v\nwant %+v", kb2, kb)
	}
}

func TestKnowledgeJSON(t *testing.T) {
	k := Knowledge{
		ID:          "doc-1",
		KBName:      "kb-1",
		FileName:    "test.pdf",
		FileType:    "pdf",
		FileSize:    1024,
		Status:      "ready",
		ChunkCount:  10,
		TokenCount:  500,
		ProcessedAt: "2023-01-01T00:00:00Z",
	}
	b, err := json.Marshal(k)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var k2 Knowledge
	if err := json.Unmarshal(b, &k2); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(k, k2) {
		t.Errorf("Mismatch:\ngot  %+v\nwant %+v", k2, k)
	}
}

func TestChunkJSON(t *testing.T) {
	c := Chunk{
		ID:         "chunk-1",
		Content:    "Hello world",
		FileName:   "test.pdf",
		PageNumber: 1,
		Score:      0.95,
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var c2 Chunk
	if err := json.Unmarshal(b, &c2); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(c, c2) {
		t.Errorf("Mismatch:\ngot  %+v\nwant %+v", c2, c)
	}

	// Test omitempty for Score
	cEmpty := Chunk{ID: "chunk-2"}
	bEmpty, _ := json.Marshal(cEmpty)
	if string(bEmpty) == "" {
		t.Fatal("Expected JSON string")
	}
	// "score" should not be present in json if it's 0 because of omitempty
	var m map[string]interface{}
	json.Unmarshal(bEmpty, &m)
	if _, ok := m["score"]; ok {
		t.Errorf("Expected 'score' to be omitted, got %v", m["score"])
	}
}

func TestSearchRequestJSON(t *testing.T) {
	sr := SearchRequest{
		KnowledgeBaseIDs: []string{"kb-1", "kb-2"},
		Query:            "what is test",
		TopK:             5,
	}
	b, err := json.Marshal(sr)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var sr2 SearchRequest
	if err := json.Unmarshal(b, &sr2); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(sr, sr2) {
		t.Errorf("Mismatch:\ngot  %+v\nwant %+v", sr2, sr)
	}
}

func TestCreateSessionRequestJSON(t *testing.T) {
	req := CreateSessionRequest{
		KnowledgeBases: []string{"kb-1"},
		AgentID:        "agent-1",
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var req2 CreateSessionRequest
	if err := json.Unmarshal(b, &req2); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(req, req2) {
		t.Errorf("Mismatch:\ngot  %+v\nwant %+v", req2, req)
	}

	// omitempty check
	emptyReq := CreateSessionRequest{}
	bEmpty, _ := json.Marshal(emptyReq)
	var m map[string]interface{}
	json.Unmarshal(bEmpty, &m)
	if _, ok := m["knowledge_bases"]; ok {
		t.Errorf("Expected 'knowledge_bases' to be omitted")
	}
	if _, ok := m["agent_id"]; ok {
		t.Errorf("Expected 'agent_id' to be omitted")
	}
}

func TestRetrievalRequestJSON(t *testing.T) {
	req := RetrievalRequest{
		Question: "how to use?",
		TopK:     3,
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var req2 RetrievalRequest
	if err := json.Unmarshal(b, &req2); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(req, req2) {
		t.Errorf("Mismatch:\ngot  %+v\nwant %+v", req2, req)
	}
}

func TestListKBResponseJSON(t *testing.T) {
	resp := ListKBResponse{
		Data: []KnowledgeBase{
			{ID: "kb-1", Name: "KB 1"},
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var resp2 ListKBResponse
	if err := json.Unmarshal(b, &resp2); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	// Note: reflect.DeepEqual might fail if time fields have 0 vs unset time.Time
	// But in our struct they will unmarshal to zero value.
	if len(resp2.Data) != 1 || resp2.Data[0].ID != "kb-1" {
		t.Errorf("Mismatch:\ngot  %+v\nwant %+v", resp2, resp)
	}
}

func TestKnowledgeListResponseJSON(t *testing.T) {
	resp := KnowledgeListResponse{
		Data: []Knowledge{
			{ID: "doc-1", FileName: "f1.pdf"},
		},
		Total: 1,
		Page:  1,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var resp2 KnowledgeListResponse
	if err := json.Unmarshal(b, &resp2); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(resp, resp2) {
		t.Errorf("Mismatch:\ngot  %+v\nwant %+v", resp2, resp)
	}
}

func TestRetrievalResponseJSON(t *testing.T) {
	resp := RetrievalResponse{
		Results: []SearchResult{
			{ChunkID: "c1", Content: "c", Score: 0.8, FileName: "f", KBName: "kb", PageNumber: 1},
		},
		Total: 1,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var resp2 RetrievalResponse
	if err := json.Unmarshal(b, &resp2); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(resp, resp2) {
		t.Errorf("Mismatch:\ngot  %+v\nwant %+v", resp2, resp)
	}
}

func TestSessionJSON(t *testing.T) {
	s := Session{ID: "s1", Name: "sesion 1"}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var s2 Session
	if err := json.Unmarshal(b, &s2); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(s, s2) {
		t.Errorf("Mismatch:\ngot  %+v\nwant %+v", s2, s)
	}
}

func TestAPIErrorJSON(t *testing.T) {
	e := APIError{Code: 404, Message: "Not found"}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var e2 APIError
	if err := json.Unmarshal(b, &e2); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(e, e2) {
		t.Errorf("Mismatch:\ngot  %+v\nwant %+v", e2, e)
	}
}

func TestAuthTypesJSON(t *testing.T) {
	// LoginRequest
	lr := LoginRequest{Email: "a@b.c", Password: "pw"}
	b, _ := json.Marshal(lr)
	var lr2 LoginRequest
	json.Unmarshal(b, &lr2)
	if !reflect.DeepEqual(lr, lr2) {
		t.Errorf("LoginRequest Mismatch")
	}

	// LoginResponse
	lresp := LoginResponse{
		Success:      true,
		Message:      "ok",
		Token:        "t1",
		RefreshToken: "t2",
		User:         &WKUser{ID: "u1", Email: "a@b.c", Username: "a"},
		Tenant:       &WKTenant{ID: 1, Name: "tnt"},
	}
	b, _ = json.Marshal(lresp)
	var lresp2 LoginResponse
	json.Unmarshal(b, &lresp2)
	if !reflect.DeepEqual(lresp, lresp2) {
		t.Errorf("LoginResponse Mismatch")
	}

	// RegisterRequest
	rr := RegisterRequest{Email: "a@b.c", Password: "pw", Username: "u"}
	b, _ = json.Marshal(rr)
	var rr2 RegisterRequest
	json.Unmarshal(b, &rr2)
	if !reflect.DeepEqual(rr, rr2) {
		t.Errorf("RegisterRequest Mismatch")
	}

	// RegisterResponse
	rresp := RegisterResponse{Success: true, Message: "ok", User: &WKUser{ID: "u1"}}
	b, _ = json.Marshal(rresp)
	var rresp2 RegisterResponse
	json.Unmarshal(b, &rresp2)
	if !reflect.DeepEqual(rresp, rresp2) {
		t.Errorf("RegisterResponse Mismatch")
	}

	// RefreshRequest
	refr := RefreshRequest{RefreshToken: "rt"}
	b, _ = json.Marshal(refr)
	var refr2 RefreshRequest
	json.Unmarshal(b, &refr2)
	if !reflect.DeepEqual(refr, refr2) {
		t.Errorf("RefreshRequest Mismatch")
	}
}

func TestKBManagementTypesJSON(t *testing.T) {
	// CreateKBRequest
	req := CreateKBRequest{Name: "kb1", Description: "d", EmbeddingModel: "m1"}
	b, _ := json.Marshal(req)
	var req2 CreateKBRequest
	json.Unmarshal(b, &req2)
	if !reflect.DeepEqual(req, req2) {
		t.Errorf("CreateKBRequest Mismatch")
	}

	// CreateKBResponse
	resp := CreateKBResponse{Success: true, Message: "ok", Data: &KnowledgeBase{ID: "kb1"}}
	b, _ = json.Marshal(resp)
	var resp2 CreateKBResponse
	json.Unmarshal(b, &resp2)
	if !reflect.DeepEqual(resp, resp2) {
		t.Errorf("CreateKBResponse Mismatch")
	}

	// DeleteKBResponse
	dresp := DeleteKBResponse{Success: true, Message: "ok"}
	b, _ = json.Marshal(dresp)
	var dresp2 DeleteKBResponse
	json.Unmarshal(b, &dresp2)
	if !reflect.DeepEqual(dresp, dresp2) {
		t.Errorf("DeleteKBResponse Mismatch")
	}
}
