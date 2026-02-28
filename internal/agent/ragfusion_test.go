package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/hflms/hanfledge/internal/infrastructure/llm"
)

// ============================
// RAG-Fusion Query Expander Tests
// ============================

// -- parseNumberedLines Tests -----------------------------------------

func TestParseNumberedLines_Standard(t *testing.T) {
	input := "1. 二次函数图像平移变换的数学原理\n2. 抛物线顶点式参数对图形的影响\n3. 二次函数图像变化规律"
	got := parseNumberedLines(input)
	if len(got) != 3 {
		t.Fatalf("expected 3 variants, got %d: %v", len(got), got)
	}
	if got[0] != "二次函数图像平移变换的数学原理" {
		t.Errorf("variant 0 = %q", got[0])
	}
}

func TestParseNumberedLines_ChineseDelimiters(t *testing.T) {
	input := "1、向量加法的几何意义\n2、向量平行四边形法则\n3、向量运算的代数表达"
	got := parseNumberedLines(input)
	if len(got) != 3 {
		t.Fatalf("expected 3 variants, got %d: %v", len(got), got)
	}
	if got[0] != "向量加法的几何意义" {
		t.Errorf("variant 0 = %q", got[0])
	}
}

func TestParseNumberedLines_ParenthesisDelimiter(t *testing.T) {
	input := "1) first query\n2) second query"
	got := parseNumberedLines(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(got))
	}
	if got[0] != "first query" {
		t.Errorf("variant 0 = %q", got[0])
	}
}

func TestParseNumberedLines_EmptyLines(t *testing.T) {
	input := "\n1. query one\n\n2. query two\n\n"
	got := parseNumberedLines(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(got))
	}
}

func TestParseNumberedLines_NoNumbers(t *testing.T) {
	input := "this is not numbered\njust plain text"
	got := parseNumberedLines(input)
	if len(got) != 0 {
		t.Fatalf("expected 0 variants, got %d: %v", len(got), got)
	}
}

func TestParseNumberedLines_Empty(t *testing.T) {
	got := parseNumberedLines("")
	if len(got) != 0 {
		t.Fatalf("expected 0 variants, got %d", len(got))
	}
}

func TestParseNumberedLines_MixedDelimiters(t *testing.T) {
	input := "1. first\n2、second\n3) third\n4: fourth"
	got := parseNumberedLines(input)
	if len(got) != 4 {
		t.Fatalf("expected 4 variants, got %d: %v", len(got), got)
	}
}

// -- stripNumberPrefix Tests ------------------------------------------

func TestStripNumberPrefix_DotDelimiter(t *testing.T) {
	got := stripNumberPrefix("1. hello world")
	if got != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", got)
	}
}

func TestStripNumberPrefix_NoDelimiter(t *testing.T) {
	got := stripNumberPrefix("1hello")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestStripNumberPrefix_NoNumber(t *testing.T) {
	got := stripNumberPrefix("hello")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestStripNumberPrefix_MultiDigit(t *testing.T) {
	got := stripNumberPrefix("12. twelfth item")
	if got != "twelfth item" {
		t.Errorf("expected %q, got %q", "twelfth item", got)
	}
}

func TestStripNumberPrefix_Empty(t *testing.T) {
	got := stripNumberPrefix("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// -- QueryExpander Tests (with mock LLM) ------------------------------

type mockExpansionLLM struct {
	response string
	err      error
}

func (m *mockExpansionLLM) Name() string { return "mock" }
func (m *mockExpansionLLM) Chat(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatOptions) (string, error) {
	return m.response, m.err
}
func (m *mockExpansionLLM) StreamChat(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatOptions, onToken func(string)) (string, error) {
	return m.response, m.err
}
func (m *mockExpansionLLM) Embed(ctx context.Context, text string) ([]float64, error) {
	return nil, nil
}
func (m *mockExpansionLLM) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	return nil, nil
}

func TestQueryExpander_ExpandQuery_Success(t *testing.T) {
	mock := &mockExpansionLLM{
		response: "1. 二次函数图像平移变换\n2. 抛物线参数影响分析\n3. 函数图像变化规律",
	}
	expander := NewQueryExpander(mock)
	queries := expander.ExpandQuery(context.Background(), "为啥抛物线变了")

	// Should contain original + 3 variants = 4 total
	if len(queries) != 4 {
		t.Fatalf("expected 4 queries, got %d: %v", len(queries), queries)
	}
	if queries[0] != "为啥抛物线变了" {
		t.Errorf("first query should be original, got %q", queries[0])
	}
}

func TestQueryExpander_ExpandQuery_LLMFailure_GracefulDegradation(t *testing.T) {
	mock := &mockExpansionLLM{
		err: fmt.Errorf("LLM service unavailable"),
	}
	expander := NewQueryExpander(mock)
	queries := expander.ExpandQuery(context.Background(), "test query")

	// Should fallback to just the original query
	if len(queries) != 1 {
		t.Fatalf("expected 1 query (original only), got %d", len(queries))
	}
	if queries[0] != "test query" {
		t.Errorf("expected original query, got %q", queries[0])
	}
}

func TestQueryExpander_ExpandQuery_NilLLM(t *testing.T) {
	expander := NewQueryExpander(nil)
	queries := expander.ExpandQuery(context.Background(), "test")

	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}
	if queries[0] != "test" {
		t.Errorf("expected %q, got %q", "test", queries[0])
	}
}

func TestQueryExpander_ExpandQuery_EmptyResponse(t *testing.T) {
	mock := &mockExpansionLLM{response: ""}
	expander := NewQueryExpander(mock)
	queries := expander.ExpandQuery(context.Background(), "original")

	// Empty LLM response → only original query
	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}
}

func TestQueryExpander_ExpandQuery_UnparsableResponse(t *testing.T) {
	mock := &mockExpansionLLM{response: "I don't understand your request."}
	expander := NewQueryExpander(mock)
	queries := expander.ExpandQuery(context.Background(), "original")

	// No numbered lines → only original query
	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}
}
