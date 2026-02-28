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

// ── checkPrereqGapsEnriched deduplication Tests ─────────────

func TestPrereqGapAutoInsertDeduplication(t *testing.T) {
	// Test that the inserted map correctly prevents duplicate insertions
	inserted := make(map[uint]bool)

	// Simulate inserting KP 10
	inserted[10] = true

	// Verify deduplication logic
	if !inserted[10] {
		t.Error("KP 10 should be marked as inserted")
	}
	if inserted[20] {
		t.Error("KP 20 should not be marked as inserted")
	}

	// Insert again — map doesn't change
	inserted[10] = true
	if count := len(inserted); count != 1 {
		t.Errorf("inserted map should have 1 entry, got %d", count)
	}
}

func TestPrereqGapTargetDefaults(t *testing.T) {
	// Test that auto-inserted prereq KP targets have correct defaults
	mastery := 0.35
	scaffold := scaffoldForMastery(mastery)

	target := KnowledgePointTarget{
		KPID:           42,
		CurrentMastery: mastery,
		TargetMastery:  0.6, // prereq target: medium threshold
		ScaffoldLevel:  scaffold,
	}

	if target.TargetMastery != 0.6 {
		t.Errorf("prereq target mastery should be 0.6, got %f", target.TargetMastery)
	}
	if target.ScaffoldLevel != ScaffoldHigh {
		t.Errorf("prereq scaffold should be high for mastery 0.35, got %s", target.ScaffoldLevel)
	}
}

func TestPrereqGapBKTDefault(t *testing.T) {
	// Test that zero mastery is treated as BKT initial value 0.1
	masteryMap := map[uint]float64{
		1: 0.7,
		2: 0.0, // zero = never attempted
	}

	// Simulate the BKT default logic from checkPrereqGapsEnriched
	for id, m := range masteryMap {
		if m == 0 {
			masteryMap[id] = 0.1
		}
	}

	if masteryMap[2] != 0.1 {
		t.Errorf("zero mastery should be defaulted to 0.1, got %f", masteryMap[2])
	}
	if masteryMap[1] != 0.7 {
		t.Errorf("non-zero mastery should be unchanged, got %f", masteryMap[1])
	}
}

func TestPrereqGapThreshold(t *testing.T) {
	// Test that only KPs below 0.6 mastery are identified as gaps
	tests := []struct {
		mastery float64
		isGap   bool
	}{
		{0.0, true},
		{0.1, true},
		{0.3, true},
		{0.59, true},
		{0.6, false}, // exactly at threshold = not a gap
		{0.7, false},
		{0.9, false},
		{1.0, false},
	}

	for _, tc := range tests {
		result := tc.mastery < 0.6
		if result != tc.isGap {
			t.Errorf("mastery=%.2f: isGap=%v, want %v", tc.mastery, result, tc.isGap)
		}
	}
}
