package agent

import (
	"math"
	"testing"
)

// ── scaffoldForMastery Tests ────────────────────────────────

func TestScaffoldForMastery(t *testing.T) {
	tests := []struct {
		name     string
		mastery  float64
		expected ScaffoldLevel
	}{
		{"掌握度0.9→低支架", 0.9, ScaffoldLow},
		{"掌握度0.8→低支架边界", 0.8, ScaffoldLow},
		{"掌握度0.79→中支架", 0.79, ScaffoldMedium},
		{"掌握度0.7→中支架", 0.7, ScaffoldMedium},
		{"掌握度0.6→中支架边界", 0.6, ScaffoldMedium},
		{"掌握度0.59→高支架", 0.59, ScaffoldHigh},
		{"掌握度0.5→高支架", 0.5, ScaffoldHigh},
		{"掌握度0.1→高支架", 0.1, ScaffoldHigh},
		{"掌握度0.0→高支架", 0.0, ScaffoldHigh},
		{"掌握度1.0→低支架", 1.0, ScaffoldLow},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := scaffoldForMastery(tc.mastery)
			if result != tc.expected {
				t.Errorf("scaffoldForMastery(%f) = %q, want %q",
					tc.mastery, result, tc.expected)
			}
		})
	}
}

// ── averageMastery Tests ────────────────────────────────────

func TestAverageMastery(t *testing.T) {
	tests := []struct {
		name     string
		targets  []KnowledgePointTarget
		expected float64
	}{
		{
			"空切片返回0.1",
			nil,
			0.1,
		},
		{
			"单个目标",
			[]KnowledgePointTarget{{CurrentMastery: 0.5}},
			0.5,
		},
		{
			"多个目标取平均",
			[]KnowledgePointTarget{
				{CurrentMastery: 0.2},
				{CurrentMastery: 0.4},
				{CurrentMastery: 0.6},
			},
			0.4,
		},
		{
			"全部掌握",
			[]KnowledgePointTarget{
				{CurrentMastery: 1.0},
				{CurrentMastery: 1.0},
			},
			1.0,
		},
		{
			"全部为零",
			[]KnowledgePointTarget{
				{CurrentMastery: 0.0},
				{CurrentMastery: 0.0},
			},
			0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := averageMastery(tc.targets)
			if math.Abs(result-tc.expected) > 1e-9 {
				t.Errorf("averageMastery() = %f, want %f", result, tc.expected)
			}
		})
	}
}

// ── sortTargetsByMastery Tests ──────────────────────────────

func TestSortTargetsByMastery(t *testing.T) {
	tests := []struct {
		name     string
		targets  []KnowledgePointTarget
		expected []float64 // expected order of CurrentMastery values
	}{
		{
			"已排序不变",
			[]KnowledgePointTarget{
				{KPID: 1, CurrentMastery: 0.1},
				{KPID: 2, CurrentMastery: 0.5},
				{KPID: 3, CurrentMastery: 0.9},
			},
			[]float64{0.1, 0.5, 0.9},
		},
		{
			"逆序排列",
			[]KnowledgePointTarget{
				{KPID: 1, CurrentMastery: 0.9},
				{KPID: 2, CurrentMastery: 0.5},
				{KPID: 3, CurrentMastery: 0.1},
			},
			[]float64{0.1, 0.5, 0.9},
		},
		{
			"单个元素",
			[]KnowledgePointTarget{
				{KPID: 1, CurrentMastery: 0.5},
			},
			[]float64{0.5},
		},
		{
			"空切片",
			[]KnowledgePointTarget{},
			[]float64{},
		},
		{
			"重复值",
			[]KnowledgePointTarget{
				{KPID: 1, CurrentMastery: 0.5},
				{KPID: 2, CurrentMastery: 0.3},
				{KPID: 3, CurrentMastery: 0.5},
			},
			[]float64{0.3, 0.5, 0.5},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sortTargetsByMastery(tc.targets)
			for i, expected := range tc.expected {
				if tc.targets[i].CurrentMastery != expected {
					t.Errorf("index %d: mastery = %f, want %f",
						i, tc.targets[i].CurrentMastery, expected)
				}
			}
		})
	}
}

// ── parseKPIDs Tests ────────────────────────────────────────

func TestParseKPIDs(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  []uint
		wantError bool
	}{
		{
			"正常JSON数组",
			"[1, 2, 3]",
			[]uint{1, 2, 3},
			false,
		},
		{
			"浮点数格式",
			"[1.0, 2.0, 3.0]",
			[]uint{1, 2, 3},
			false,
		},
		{
			"单个元素",
			"[42]",
			[]uint{42},
			false,
		},
		{
			"空字符串报错",
			"",
			nil,
			true,
		},
		{
			"null报错",
			"null",
			nil,
			true,
		},
		{
			"非法JSON报错",
			"{not json}",
			nil,
			true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseKPIDs(tc.input)
			if tc.wantError {
				if err == nil {
					t.Errorf("parseKPIDs(%q) should return error", tc.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parseKPIDs(%q) returned unexpected error: %v", tc.input, err)
				return
			}
			if len(result) != len(tc.expected) {
				t.Errorf("parseKPIDs(%q) returned %d items, want %d",
					tc.input, len(result), len(tc.expected))
				return
			}
			for i, v := range tc.expected {
				if result[i] != v {
					t.Errorf("parseKPIDs(%q)[%d] = %d, want %d",
						tc.input, i, result[i], v)
				}
			}
		})
	}
}
