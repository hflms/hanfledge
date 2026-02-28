package handler

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/plugin"
	"gorm.io/gorm"
)

// ============================
// Custom Skill Handler Tests
// ============================

// seedCustomSkill creates a custom skill in the database and returns it.
func seedCustomSkill(t *testing.T, db *gorm.DB, teacherID uint, skillID, name string, status model.CustomSkillStatus) model.CustomSkill {
	t.Helper()
	s := model.CustomSkill{
		SkillID:     skillID,
		TeacherID:   teacherID,
		Name:        name,
		Description: "测试技能描述",
		Category:    "inquiry-based",
		Subjects:    `["math"]`,
		Tags:        `["test"]`,
		SkillMD:     "# Test Skill\nYou are a Socratic tutor.",
		ToolsConfig: `{}`,
		Templates:   `[]`,
		Status:      status,
		Visibility:  model.VisibilityPrivate,
		Version:     1,
		CreatedAt:   "2025-01-01T00:00:00Z",
		UpdatedAt:   "2025-01-01T00:00:00Z",
	}
	if err := db.Create(&s).Error; err != nil {
		t.Fatalf("seedCustomSkill failed: %v", err)
	}
	return s
}

// -- CreateCustomSkill Tests ----------------------------------------

func TestCustomSkillHandler_CreateCustomSkill(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800010001", "pass123", "张老师", model.UserStatusActive)
	registry := plugin.NewRegistry()
	h := NewCustomSkillHandler(db, registry)

	t.Run("creates skill in draft status", func(t *testing.T) {
		body := `{
			"skill_id": "math_concept_analogy",
			"name": "数学类比教学",
			"description": "通过类比方式教授数学概念",
			"category": "inquiry-based",
			"subjects": ["math"],
			"tags": ["analogy"],
			"skill_md": "# Math Analogy\nUse analogies to explain."
		}`
		w, c := newTestContext("POST", "/api/v1/custom-skills", body, teacher.ID)
		h.CreateCustomSkill(c)
		assertStatus(t, w, http.StatusCreated)
		assertBodyContains(t, w, "math_concept_analogy")
		assertBodyContains(t, w, "draft")
	})

	t.Run("rejects invalid namespace (less than 3 segments)", func(t *testing.T) {
		body := `{
			"skill_id": "math_analogy",
			"name": "短ID",
			"skill_md": "# Short"
		}`
		w, c := newTestContext("POST", "/api/v1/custom-skills", body, teacher.ID)
		h.CreateCustomSkill(c)
		assertStatus(t, w, http.StatusBadRequest)
		assertBodyContains(t, w, "技能 ID 格式错误")
	})

	t.Run("rejects invalid namespace (uppercase)", func(t *testing.T) {
		body := `{
			"skill_id": "Math_concept_analogy",
			"name": "大写",
			"skill_md": "# Upper"
		}`
		w, c := newTestContext("POST", "/api/v1/custom-skills", body, teacher.ID)
		h.CreateCustomSkill(c)
		assertStatus(t, w, http.StatusBadRequest)
		assertBodyContains(t, w, "技能 ID 格式错误")
	})

	t.Run("rejects duplicate skill ID", func(t *testing.T) {
		body := `{
			"skill_id": "math_concept_analogy",
			"name": "重复技能",
			"skill_md": "# Dup"
		}`
		w, c := newTestContext("POST", "/api/v1/custom-skills", body, teacher.ID)
		h.CreateCustomSkill(c)
		assertStatus(t, w, http.StatusConflict)
		assertBodyContains(t, w, "技能 ID 已存在")
	})

	t.Run("rejects skill ID already in registry", func(t *testing.T) {
		registry.RegisterSkillWithMetadata(plugin.SkillMetadata{
			ID:   "physics_lab_experiment",
			Name: "物理实验",
		})
		body := `{
			"skill_id": "physics_lab_experiment",
			"name": "冲突技能",
			"skill_md": "# Conflict"
		}`
		w, c := newTestContext("POST", "/api/v1/custom-skills", body, teacher.ID)
		h.CreateCustomSkill(c)
		assertStatus(t, w, http.StatusConflict)
		assertBodyContains(t, w, "技能 ID 已存在")
	})

	t.Run("rejects missing required fields", func(t *testing.T) {
		body := `{"name": "无ID"}`
		w, c := newTestContext("POST", "/api/v1/custom-skills", body, teacher.ID)
		h.CreateCustomSkill(c)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("rejects SKILL.md exceeding token limit", func(t *testing.T) {
		// Generate a very long SKILL.md (>2000 tokens ≈ ~8000 English chars)
		longMD := ""
		for i := 0; i < 10000; i++ {
			longMD += "word "
		}
		body := fmt.Sprintf(`{
			"skill_id": "math_long_skillmd",
			"name": "超长约束",
			"skill_md": %q
		}`, longMD)
		w, c := newTestContext("POST", "/api/v1/custom-skills", body, teacher.ID)
		h.CreateCustomSkill(c)
		assertStatus(t, w, http.StatusBadRequest)
		assertBodyContains(t, w, "Token 限制")
	})
}

// -- ListCustomSkills Tests ----------------------------------------

func TestCustomSkillHandler_ListCustomSkills(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800020001", "pass123", "李老师", model.UserStatusActive)
	otherTeacher := seedUser(t, db, "13800020002", "pass123", "王老师", model.UserStatusActive)
	registry := plugin.NewRegistry()
	h := NewCustomSkillHandler(db, registry)

	seedCustomSkill(t, db, teacher.ID, "math_quiz_basic", "数学测验基础", model.CustomSkillStatusDraft)
	seedCustomSkill(t, db, teacher.ID, "math_quiz_advanced", "数学测验进阶", model.CustomSkillStatusPublished)
	seedCustomSkill(t, db, otherTeacher.ID, "physics_lab_demo", "物理实验演示", model.CustomSkillStatusDraft)

	t.Run("lists only own skills", func(t *testing.T) {
		w, c := newTestContextWithQuery("GET", "/api/v1/custom-skills", teacher.ID)
		h.ListCustomSkills(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "数学测验基础")
		assertBodyContains(t, w, "数学测验进阶")
		assertBodyNotContains(t, w, "物理实验演示")
	})

	t.Run("filters by status", func(t *testing.T) {
		w, c := newTestContextWithQuery("GET", "/api/v1/custom-skills?status=draft", teacher.ID)
		h.ListCustomSkills(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "数学测验基础")
		assertBodyNotContains(t, w, "数学测验进阶")
	})

	t.Run("returns empty for teacher with no skills", func(t *testing.T) {
		newTeacher := seedUser(t, db, "13800020003", "pass123", "赵老师", model.UserStatusActive)
		w, c := newTestContextWithQuery("GET", "/api/v1/custom-skills", newTeacher.ID)
		h.ListCustomSkills(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "[]")
	})
}

// -- GetCustomSkill Tests ----------------------------------------

func TestCustomSkillHandler_GetCustomSkill(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800030001", "pass123", "陈老师", model.UserStatusActive)
	other := seedUser(t, db, "13800030002", "pass123", "周老师", model.UserStatusActive)
	registry := plugin.NewRegistry()
	h := NewCustomSkillHandler(db, registry)

	skill := seedCustomSkill(t, db, teacher.ID, "math_calc_drill", "计算练习", model.CustomSkillStatusDraft)

	t.Run("returns own skill", func(t *testing.T) {
		w, c := newTestContextWithParams("GET", "/api/v1/custom-skills/1", "",
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.GetCustomSkill(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "计算练习")
		assertBodyContains(t, w, "math_calc_drill")
	})

	t.Run("forbids access to other teacher's private skill", func(t *testing.T) {
		w, c := newTestContextWithParams("GET", "/api/v1/custom-skills/1", "",
			other.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.GetCustomSkill(c)
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("returns 404 for non-existent skill", func(t *testing.T) {
		w, c := newTestContextWithParams("GET", "/api/v1/custom-skills/999", "",
			teacher.ID, gin.Params{{Key: "id", Value: "999"}})
		h.GetCustomSkill(c)
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("allows access to shared skill by other teacher", func(t *testing.T) {
		sharedSkill := seedCustomSkill(t, db, teacher.ID, "math_shared_demo", "共享技能", model.CustomSkillStatusShared)
		// Update visibility to school
		db.Model(&sharedSkill).Update("visibility", model.VisibilitySchool)

		w, c := newTestContextWithParams("GET", "/api/v1/custom-skills/2", "",
			other.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", sharedSkill.ID)}})
		h.GetCustomSkill(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "共享技能")
	})
}

// -- UpdateCustomSkill Tests ----------------------------------------

func TestCustomSkillHandler_UpdateCustomSkill(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800040001", "pass123", "吴老师", model.UserStatusActive)
	other := seedUser(t, db, "13800040002", "pass123", "郑老师", model.UserStatusActive)
	registry := plugin.NewRegistry()
	h := NewCustomSkillHandler(db, registry)

	t.Run("updates draft skill fields", func(t *testing.T) {
		skill := seedCustomSkill(t, db, teacher.ID, "math_update_test1", "原名", model.CustomSkillStatusDraft)
		body := `{"name": "新名称", "description": "新描述"}`
		w, c := newTestContextWithParams("PUT", "/api/v1/custom-skills/1", body,
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.UpdateCustomSkill(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "新名称")
		assertBodyContains(t, w, "新描述")
	})

	t.Run("creates version history when updating published skill", func(t *testing.T) {
		skill := seedCustomSkill(t, db, teacher.ID, "math_update_test2", "发布版", model.CustomSkillStatusPublished)
		// Register in registry so update path works
		registry.RegisterCustomSkill(plugin.SkillMetadata{ID: skill.SkillID, Name: skill.Name}, skill.SkillMD)

		body := `{"skill_md": "# Updated SKILL.md", "change_log": "修改约束规则"}`
		w, c := newTestContextWithParams("PUT", "/api/v1/custom-skills/2", body,
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.UpdateCustomSkill(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, `"version":2`)

		// Verify version history was saved
		var versions []model.CustomSkillVersion
		db.Where("custom_skill_id = ?", skill.ID).Find(&versions)
		if len(versions) != 1 {
			t.Errorf("expected 1 version record, got %d", len(versions))
		}
		if versions[0].Version != 1 {
			t.Errorf("version = %d, want 1", versions[0].Version)
		}
	})

	t.Run("forbids update by non-owner", func(t *testing.T) {
		body := `{"name": "偷改"}`
		w, c := newTestContextWithParams("PUT", "/api/v1/custom-skills/1", body,
			other.ID, gin.Params{{Key: "id", Value: "1"}})
		h.UpdateCustomSkill(c)
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("rejects SKILL.md exceeding token limit", func(t *testing.T) {
		longMD := ""
		for i := 0; i < 10000; i++ {
			longMD += "word "
		}
		body := fmt.Sprintf(`{"skill_md": %q}`, longMD)
		w, c := newTestContextWithParams("PUT", "/api/v1/custom-skills/1", body,
			teacher.ID, gin.Params{{Key: "id", Value: "1"}})
		h.UpdateCustomSkill(c)
		assertStatus(t, w, http.StatusBadRequest)
		assertBodyContains(t, w, "Token 限制")
	})
}

// -- DeleteCustomSkill Tests ----------------------------------------

func TestCustomSkillHandler_DeleteCustomSkill(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800050001", "pass123", "孙老师", model.UserStatusActive)
	other := seedUser(t, db, "13800050002", "pass123", "钱老师", model.UserStatusActive)
	registry := plugin.NewRegistry()
	h := NewCustomSkillHandler(db, registry)

	t.Run("deletes draft skill", func(t *testing.T) {
		skill := seedCustomSkill(t, db, teacher.ID, "math_delete_draft", "待删草稿", model.CustomSkillStatusDraft)
		w, c := newTestContextWithParams("DELETE", "/api/v1/custom-skills/1", "",
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.DeleteCustomSkill(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "技能已删除")

		// Verify actually deleted
		var count int64
		db.Model(&model.CustomSkill{}).Where("id = ?", skill.ID).Count(&count)
		if count != 0 {
			t.Error("skill should be deleted from DB")
		}
	})

	t.Run("rejects deleting published skill", func(t *testing.T) {
		skill := seedCustomSkill(t, db, teacher.ID, "math_delete_pub", "已发布", model.CustomSkillStatusPublished)
		w, c := newTestContextWithParams("DELETE", "/api/v1/custom-skills/2", "",
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.DeleteCustomSkill(c)
		assertStatus(t, w, http.StatusBadRequest)
		assertBodyContains(t, w, "已发布的技能不能直接删除")
	})

	t.Run("forbids deletion by non-owner", func(t *testing.T) {
		skill := seedCustomSkill(t, db, teacher.ID, "math_delete_other", "别人的", model.CustomSkillStatusDraft)
		w, c := newTestContextWithParams("DELETE", "/api/v1/custom-skills/3", "",
			other.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.DeleteCustomSkill(c)
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("deletes archived skill", func(t *testing.T) {
		skill := seedCustomSkill(t, db, teacher.ID, "math_delete_arch", "已归档", model.CustomSkillStatusArchived)
		w, c := newTestContextWithParams("DELETE", "/api/v1/custom-skills/4", "",
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.DeleteCustomSkill(c)
		assertStatus(t, w, http.StatusOK)
	})
}

// -- PublishCustomSkill Tests ----------------------------------------

func TestCustomSkillHandler_PublishCustomSkill(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800060001", "pass123", "何老师", model.UserStatusActive)
	registry := plugin.NewRegistry()
	h := NewCustomSkillHandler(db, registry)

	t.Run("publishes draft skill and registers in registry", func(t *testing.T) {
		skill := seedCustomSkill(t, db, teacher.ID, "math_publish_ok", "可发布", model.CustomSkillStatusDraft)
		w, c := newTestContextWithParams("POST", "/api/v1/custom-skills/1/publish", "",
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.PublishCustomSkill(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "技能已发布")
		assertBodyContains(t, w, "published")

		// Verify registered in plugin registry
		regSkill, ok := registry.GetSkill("math_publish_ok")
		if !ok {
			t.Fatal("skill not found in registry after publish")
		}
		if !regSkill.IsCustom {
			t.Error("registered skill should be marked as custom")
		}
		if regSkill.SkillMDContent == "" {
			t.Error("registered skill should have SkillMDContent")
		}
	})

	t.Run("rejects publishing already published skill", func(t *testing.T) {
		skill := seedCustomSkill(t, db, teacher.ID, "math_publish_dup", "已发布的", model.CustomSkillStatusPublished)
		w, c := newTestContextWithParams("POST", "/api/v1/custom-skills/2/publish", "",
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.PublishCustomSkill(c)
		assertStatus(t, w, http.StatusBadRequest)
		assertBodyContains(t, w, "只有草稿状态")
	})

	t.Run("rejects publishing skill with empty SKILL.md", func(t *testing.T) {
		skill := model.CustomSkill{
			SkillID:     "math_publish_empty",
			TeacherID:   teacher.ID,
			Name:        "空约束",
			SkillMD:     "",
			Status:      model.CustomSkillStatusDraft,
			Visibility:  model.VisibilityPrivate,
			Version:     1,
			Subjects:    `[]`,
			Tags:        `[]`,
			ToolsConfig: `{}`,
			Templates:   `[]`,
			CreatedAt:   "2025-01-01T00:00:00Z",
			UpdatedAt:   "2025-01-01T00:00:00Z",
		}
		db.Create(&skill)

		w, c := newTestContextWithParams("POST", "/api/v1/custom-skills/3/publish", "",
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.PublishCustomSkill(c)
		assertStatus(t, w, http.StatusBadRequest)
		assertBodyContains(t, w, "不能为空")
	})
}

// -- ShareCustomSkill Tests ----------------------------------------

func TestCustomSkillHandler_ShareCustomSkill(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800070001", "pass123", "冯老师", model.UserStatusActive)
	registry := plugin.NewRegistry()
	h := NewCustomSkillHandler(db, registry)

	t.Run("shares published skill to school", func(t *testing.T) {
		skill := seedCustomSkill(t, db, teacher.ID, "math_share_school", "可分享", model.CustomSkillStatusPublished)
		body := `{"visibility": "school"}`
		w, c := newTestContextWithParams("POST", "/api/v1/custom-skills/1/share", body,
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.ShareCustomSkill(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "技能已分享")
		assertBodyContains(t, w, "school")
	})

	t.Run("shares to platform", func(t *testing.T) {
		skill := seedCustomSkill(t, db, teacher.ID, "math_share_platform", "平台分享", model.CustomSkillStatusPublished)
		body := `{"visibility": "platform"}`
		w, c := newTestContextWithParams("POST", "/api/v1/custom-skills/2/share", body,
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.ShareCustomSkill(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "platform")
	})

	t.Run("rejects sharing draft skill", func(t *testing.T) {
		skill := seedCustomSkill(t, db, teacher.ID, "math_share_draft", "草稿", model.CustomSkillStatusDraft)
		body := `{"visibility": "school"}`
		w, c := newTestContextWithParams("POST", "/api/v1/custom-skills/3/share", body,
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.ShareCustomSkill(c)
		assertStatus(t, w, http.StatusBadRequest)
		assertBodyContains(t, w, "只有已发布")
	})

	t.Run("rejects invalid visibility", func(t *testing.T) {
		skill := seedCustomSkill(t, db, teacher.ID, "math_share_bad", "错误可见", model.CustomSkillStatusPublished)
		body := `{"visibility": "private"}`
		w, c := newTestContextWithParams("POST", "/api/v1/custom-skills/4/share", body,
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.ShareCustomSkill(c)
		assertStatus(t, w, http.StatusBadRequest)
		assertBodyContains(t, w, "school 或 platform")
	})
}

// -- ArchiveCustomSkill Tests ----------------------------------------

func TestCustomSkillHandler_ArchiveCustomSkill(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800080001", "pass123", "褚老师", model.UserStatusActive)
	registry := plugin.NewRegistry()
	h := NewCustomSkillHandler(db, registry)

	t.Run("archives published skill and unregisters from registry", func(t *testing.T) {
		skill := seedCustomSkill(t, db, teacher.ID, "math_archive_ok", "待归档", model.CustomSkillStatusPublished)
		// Register in registry first
		registry.RegisterCustomSkill(plugin.SkillMetadata{ID: skill.SkillID, Name: skill.Name}, skill.SkillMD)

		w, c := newTestContextWithParams("POST", "/api/v1/custom-skills/1/archive", "",
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.ArchiveCustomSkill(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "技能已归档")
		assertBodyContains(t, w, "archived")

		// Verify unregistered from registry
		if _, ok := registry.GetSkill("math_archive_ok"); ok {
			t.Error("skill should be removed from registry after archive")
		}
	})

	t.Run("rejects archiving already archived skill", func(t *testing.T) {
		skill := seedCustomSkill(t, db, teacher.ID, "math_archive_dup", "已归档", model.CustomSkillStatusArchived)
		w, c := newTestContextWithParams("POST", "/api/v1/custom-skills/2/archive", "",
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.ArchiveCustomSkill(c)
		assertStatus(t, w, http.StatusBadRequest)
		assertBodyContains(t, w, "已经是归档状态")
	})
}

// -- ListVersions Tests ----------------------------------------

func TestCustomSkillHandler_ListVersions(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800090001", "pass123", "卫老师", model.UserStatusActive)
	registry := plugin.NewRegistry()
	h := NewCustomSkillHandler(db, registry)

	skill := seedCustomSkill(t, db, teacher.ID, "math_version_test", "版本测试", model.CustomSkillStatusPublished)

	t.Run("returns empty when no version history", func(t *testing.T) {
		w, c := newTestContextWithParams("GET", "/api/v1/custom-skills/1/versions", "",
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.ListVersions(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "[]")
	})

	t.Run("returns version history after updates", func(t *testing.T) {
		// Seed version records
		db.Create(&model.CustomSkillVersion{
			CustomSkillID: skill.ID,
			Version:       1,
			SkillMD:       "# V1 content",
			ToolsConfig:   "{}",
			Templates:     "[]",
			ChangeLog:     "初始版本",
			CreatedAt:     "2025-01-01T00:00:00Z",
		})
		db.Create(&model.CustomSkillVersion{
			CustomSkillID: skill.ID,
			Version:       2,
			SkillMD:       "# V2 content",
			ToolsConfig:   "{}",
			Templates:     "[]",
			ChangeLog:     "修改约束",
			CreatedAt:     "2025-01-02T00:00:00Z",
		})

		w, c := newTestContextWithParams("GET", "/api/v1/custom-skills/1/versions", "",
			teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.ListVersions(c)
		assertStatus(t, w, http.StatusOK)
		assertBodyContains(t, w, "初始版本")
		assertBodyContains(t, w, "修改约束")
	})

	t.Run("forbids access by non-owner", func(t *testing.T) {
		other := seedUser(t, db, "13800090002", "pass123", "蒋老师", model.UserStatusActive)
		w, c := newTestContextWithParams("GET", "/api/v1/custom-skills/1/versions", "",
			other.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", skill.ID)}})
		h.ListVersions(c)
		assertStatus(t, w, http.StatusForbidden)
	})
}

// -- estimateTokens Tests ----------------------------------------

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		minTokens int
		maxTokens int
	}{
		{"empty string", "", 0, 2},
		{"pure english short", "Hello world", 2, 5},
		{"pure chinese", "你好世界", 2, 4},
		{"mixed content", "# 技能标题\nUse Socratic method.", 5, 20},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens := estimateTokens(tc.text)
			if tokens < tc.minTokens || tokens > tc.maxTokens {
				t.Errorf("estimateTokens(%q) = %d, want [%d, %d]",
					tc.text, tokens, tc.minTokens, tc.maxTokens)
			}
		})
	}
}

// -- Constructor Test ----------------------------------------

func TestNewCustomSkillHandler(t *testing.T) {
	h := NewCustomSkillHandler(nil, nil)
	if h == nil {
		t.Fatal("NewCustomSkillHandler returned nil")
	}
	if h.DB != nil {
		t.Error("expected nil DB")
	}
	if h.Registry != nil {
		t.Error("expected nil Registry")
	}
}
