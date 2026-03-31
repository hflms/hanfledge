package lms

import (
"context"
"crypto/rand"
"crypto/rsa"
"crypto/x509"
"encoding/json"
"encoding/pem"
"net/http"
"net/http/httptest"
"testing"
"time"
)

func setupMockServers(t *testing.T) (*httptest.Server, *httptest.Server) {
tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodPost {
t.Errorf("Expected token request to be POST, got %s", r.Method)
}

err := r.ParseForm()
if err != nil {
t.Errorf("Failed to parse form: %v", err)
}

if r.FormValue("grant_type") != "client_credentials" {
t.Errorf("Expected grant_type to be client_credentials")
}

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]string{
"access_token": "mock-access-token",
"token_type":   "Bearer",
})
}))

scoreServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodPost {
t.Errorf("Expected score request to be POST, got %s", r.Method)
}

if r.Header.Get("Authorization") != "Bearer mock-access-token" {
t.Errorf("Expected Bearer mock-access-token, got %s", r.Header.Get("Authorization"))
}

if r.Header.Get("Content-Type") != "application/vnd.ims.lis.v1.score+json" {
t.Errorf("Expected correct Content-Type, got %s", r.Header.Get("Content-Type"))
}

var score agsScore
if err := json.NewDecoder(r.Body).Decode(&score); err != nil {
t.Errorf("Failed to decode score payload: %v", err)
}

if score.ScoreGiven != 0.95 {
t.Errorf("Expected ScoreGiven 0.95, got %f", score.ScoreGiven)
}

w.WriteHeader(http.StatusOK)
}))

return tokenServer, scoreServer
}

func TestLTI13Adapter_ReportScore(t *testing.T) {
testPrivateKey := generateTestPrivateKeyPEM(t)
tokenServer, scoreServer := setupMockServers(t)
defer tokenServer.Close()
defer scoreServer.Close()

cfg := LMSConfig{
Type:        AdapterLTI13,
ClientID:    "test-client-id",
PlatformURL: "https://test.platform.com",
TokenURL:    tokenServer.URL,
PrivateKey:  testPrivateKey,
}

adapter, err := NewLTI13Adapter(cfg)
if err != nil {
t.Fatalf("failed to create LTI13Adapter: %v", err)
}

req := ScoreReport{
UserID:     "user-123",
CourseID:   "course-456",
ActivityID: scoreServer.URL,
Score:      0.95,
MaxScore:   1.0,
Comment:    "Great job!",
Timestamp:  time.Now(),
Status:     "completed",
}

ctx := context.Background()
err = adapter.ReportScore(ctx, req)
if err != nil {
t.Errorf("ReportScore failed: %v", err)
}
}

func TestLTI13Adapter_ReportScore_InvalidKey(t *testing.T) {
tokenServer, scoreServer := setupMockServers(t)
defer tokenServer.Close()
defer scoreServer.Close()

cfg := LMSConfig{
Type:        AdapterLTI13,
ClientID:    "test-client-id",
PlatformURL: "https://test.platform.com",
TokenURL:    tokenServer.URL,
PrivateKey:  "invalid-private-key",
}

adapter, err := NewLTI13Adapter(cfg)
if err != nil {
t.Fatalf("failed to create LTI13Adapter: %v", err)
}

req := ScoreReport{
UserID:     "user-123",
CourseID:   "course-456",
ActivityID: scoreServer.URL,
Score:      0.95,
MaxScore:   1.0,
Comment:    "Great job!",
Timestamp:  time.Now(),
Status:     "completed",
}

ctx := context.Background()
err = adapter.ReportScore(ctx, req)
if err == nil {
t.Errorf("Expected ReportScore to fail with invalid private key")
}
}

func TestSyncRoster(t *testing.T) {
// Generate a dummy RSA private key for testing
privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
if err != nil {
t.Fatalf("Failed to generate RSA key: %v", err)
}

privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
pemBlock := &pem.Block{
Type:  "RSA PRIVATE KEY",
Bytes: privBytes,
}
privKeyPEM := string(pem.EncodeToMemory(pemBlock))

// Mock token server
tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodPost {
t.Errorf("Expected POST request for token, got %s", r.Method)
}

err := r.ParseForm()
if err != nil {
t.Errorf("Failed to parse form: %v", err)
}

if r.FormValue("grant_type") != "client_credentials" {
t.Errorf("Expected grant_type=client_credentials, got %s", r.FormValue("grant_type"))
}

if r.FormValue("scope") != "https://purl.imsglobal.org/spec/lti-nrps/scope/contextmembership.readonly" {
t.Errorf("Unexpected scope: %s", r.FormValue("scope"))
}

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]interface{}{
"access_token": "mock-token-123",
"token_type":   "Bearer",
"expires_in":   3600,
})
}))
defer tokenServer.Close()

// Mock NRPS server
nrpsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodGet {
t.Errorf("Expected GET request for NRPS, got %s", r.Method)
}

if auth := r.Header.Get("Authorization"); auth != "Bearer mock-token-123" {
t.Errorf("Expected 'Bearer mock-token-123' Authorization header, got %s", auth)
}

if accept := r.Header.Get("Accept"); accept != "application/vnd.ims.lti-nrps.v2.membershipcontainer+json" {
t.Errorf("Unexpected Accept header: %s", accept)
}

w.Header().Set("Content-Type", "application/vnd.ims.lti-nrps.v2.membershipcontainer+json")

page := r.URL.Query().Get("page")

if page == "" {
// First page
scheme := "http"
if r.TLS != nil {
scheme = "https"
}
w.Header().Set("Link", "<"+scheme+"://"+r.Host+r.URL.Path+"?page=2>; rel=\"next\"")
mockResponse := NRPSResponse{
ID: "mock-nrps-response-id",
Context: NRPSContext{
ID:    "course-123",
Label: "CS101",
Title: "Intro to Computer Science",
},
Members: []NRPSMember{
{
Status: "Active",
Name:   "Jane Doe",
Email:  "jane@example.com",
UserID: "user-1",
Roles:  []string{"http://purl.imsglobal.org/vocab/lis/v2/membership#Learner"},
},
{
Status: "Active",
Name:   "John Smith",
Email:  "john@example.com",
UserID: "user-2",
Roles:  []string{"http://purl.imsglobal.org/vocab/lis/v2/membership#Instructor"},
},
},
}
json.NewEncoder(w).Encode(mockResponse)
} else if page == "2" {
// Second page
mockResponse := NRPSResponse{
ID: "mock-nrps-response-id-2",
Context: NRPSContext{
ID:    "course-123",
Label: "CS101",
Title: "Intro to Computer Science",
},
Members: []NRPSMember{
{
Status: "Deleted",
Name:   "Inactive User",
Email:  "inactive@example.com",
UserID: "user-3",
Roles:  []string{"http://purl.imsglobal.org/vocab/lis/v2/membership#Learner"},
},
},
}
json.NewEncoder(w).Encode(mockResponse)
} else {
w.WriteHeader(http.StatusNotFound)
}
}))
defer nrpsServer.Close()

cfg := LMSConfig{
Type:        AdapterLTI13,
ClientID:    "client-id-123",
PlatformURL: "https://platform.example.com",
TokenURL:    tokenServer.URL,
PrivateKey:  privKeyPEM,
}

adapter, err := NewLTI13Adapter(cfg)
if err != nil {
t.Fatalf("Failed to create adapter: %v", err)
}

// In LTI 1.3 NRPS implementation, courseID acts as the memberships URL
roster, err := adapter.SyncRoster(context.Background(), nrpsServer.URL)
if err != nil {
t.Fatalf("SyncRoster failed: %v", err)
}

if roster.CourseID != "course-123" {
t.Errorf("Expected CourseID 'course-123', got %s", roster.CourseID)
}

if len(roster.Members) != 3 {
t.Fatalf("Expected 3 members, got %d", len(roster.Members))
}

// Verify Jane Doe (Student)
if roster.Members[0].Role != "student" {
t.Errorf("Expected Jane Doe to be 'student', got %s", roster.Members[0].Role)
}
if roster.Members[0].Status != "active" {
t.Errorf("Expected Jane Doe to be 'active', got %s", roster.Members[0].Status)
}

// Verify John Smith (Teacher)
if roster.Members[1].Role != "teacher" {
t.Errorf("Expected John Smith to be 'teacher', got %s", roster.Members[1].Role)
}
if roster.Members[1].Status != "active" {
t.Errorf("Expected John Smith to be 'active', got %s", roster.Members[1].Status)
}

// Verify Inactive User
if roster.Members[2].Role != "student" {
t.Errorf("Expected Inactive User to be 'student', got %s", roster.Members[2].Role)
}
if roster.Members[2].Status != "inactive" {
t.Errorf("Expected Inactive User to be 'inactive', got %s", roster.Members[2].Status)
}
}
