package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/plugin"
)

// ============================
// Skill KP Handler Unit Tests
// ============================

// newTestSkillHandler creates a SkillHandler with a real DB and a test registry.
func newTestSkillHandler(t *testing.T) *SkillHandler {
	t.Helper()
	db := setupTestDB(t)
	registry := plugin.NewRegistry()
	return NewSkillHandler(db, registry, nil)
}

// registerTestSkill adds a skill to the registry for testing.
func registerTestSkill(registry *plugin.Registry, id string) {
	registry.RegisterSkillWithMetadata(plugin.SkillMetadata{
		ID:      id,
		Name:    "Test Skill " + id,
		Version: "1.0.0",
	})
}

// -- MountSkillToKP Tests -------------------------------------

func TestMountSkillToKP_InvalidKPID(t *testing.T) {
	h := newTestSkillHandler(t)

	body := `{"skill_id":"test-skill"}`
	w, c := newTestContextWithParams("POST", "/api/v1/knowledge-points/abc/skills", body, 1,
		gin.Params{{Key: "id", Value: "abc"}})
	h.MountSkillToKP(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "无效的知识点 ID")
}

func TestMountSkillToKP_InvalidJSON(t *testing.T) {
	h := newTestSkillHandler(t)

	w, c := newTestContextWithParams("POST", "/api/v1/knowledge-points/1/skills", "not json", 1,
		gin.Params{{Key: "id", Value: "1"}})
	h.MountSkillToKP(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "请求数据格式错误")
}

func TestMountSkillToKP_SkillNotInRegistry(t *testing.T) {
	h := newTestSkillHandler(t)

	body := `{"skill_id":"nonexistent-skill"}`
	w, c := newTestContextWithParams("POST", "/api/v1/knowledge-points/1/skills", body, 1,
		gin.Params{{Key: "id", Value: "1"}})
	h.MountSkillToKP(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "技能不存在")
}

func TestMountSkillToKP_KPNotFound(t *testing.T) {
	h := newTestSkillHandler(t)
	registerTestSkill(h.Registry, "test-skill")

	body := `{"skill_id":"test-skill"}`
	w, c := newTestContextWithParams("POST", "/api/v1/knowledge-points/9999/skills", body, 1,
		gin.Params{{Key: "id", Value: "9999"}})
	h.MountSkillToKP(c)

	assertStatus(t, w, http.StatusNotFound)
	assertBodyContains(t, w, "知识点不存在")
}

func TestMountSkillToKP_Success(t *testing.T) {
	h := newTestSkillHandler(t)
	registerTestSkill(h.Registry, "test-skill")

	// Seed a course, chapter, and KP
	teacher := seedUser(t, h.DB, "13800000001", "pass123", "Teacher", model.UserStatusActive)
	course := seedCourse(t, h.DB, teacher.ID, "Physics")
	chapter := seedChapter(t, h.DB, course.ID, "Chapter 1", 1)
	kp := seedKP(t, h.DB, chapter.ID, "KP 1")

	body := `{"skill_id":"test-skill"}`
	w, c := newTestContextWithParams("POST", "/api/v1/knowledge-points/"+itoa(int(kp.ID))+"/skills", body, 1,
		gin.Params{{Key: "id", Value: itoa(int(kp.ID))}})
	h.MountSkillToKP(c)

	assertStatus(t, w, http.StatusCreated)
	assertBodyContains(t, w, "技能挂载成功")
	assertBodyContains(t, w, "test-skill")
}

func TestMountSkillToKP_DefaultScaffoldLevel(t *testing.T) {
	h := newTestSkillHandler(t)
	registerTestSkill(h.Registry, "test-skill")

	teacher := seedUser(t, h.DB, "13800000001", "pass123", "Teacher", model.UserStatusActive)
	course := seedCourse(t, h.DB, teacher.ID, "Physics")
	chapter := seedChapter(t, h.DB, course.ID, "Chapter 1", 1)
	kp := seedKP(t, h.DB, chapter.ID, "KP 1")

	body := `{"skill_id":"test-skill"}`
	w, c := newTestContextWithParams("POST", "/api/v1/knowledge-points/"+itoa(int(kp.ID))+"/skills", body, 1,
		gin.Params{{Key: "id", Value: itoa(int(kp.ID))}})
	h.MountSkillToKP(c)

	assertStatus(t, w, http.StatusCreated)

	// Verify default scaffold level is "high"
	var mount model.KPSkillMount
	h.DB.Where("kp_id = ?", kp.ID).First(&mount)
	if mount.ScaffoldLevel != model.ScaffoldHigh {
		t.Errorf("ScaffoldLevel = %q, want %q", mount.ScaffoldLevel, model.ScaffoldHigh)
	}
}

func TestMountSkillToKP_DuplicateMount(t *testing.T) {
	h := newTestSkillHandler(t)
	registerTestSkill(h.Registry, "test-skill")

	teacher := seedUser(t, h.DB, "13800000001", "pass123", "Teacher", model.UserStatusActive)
	course := seedCourse(t, h.DB, teacher.ID, "Physics")
	chapter := seedChapter(t, h.DB, course.ID, "Chapter 1", 1)
	kp := seedKP(t, h.DB, chapter.ID, "KP 1")

	// First mount
	body := `{"skill_id":"test-skill"}`
	w1, c1 := newTestContextWithParams("POST", "/api/v1/knowledge-points/"+itoa(int(kp.ID))+"/skills", body, 1,
		gin.Params{{Key: "id", Value: itoa(int(kp.ID))}})
	h.MountSkillToKP(c1)
	assertStatus(t, w1, http.StatusCreated)

	// Duplicate mount
	w2, c2 := newTestContextWithParams("POST", "/api/v1/knowledge-points/"+itoa(int(kp.ID))+"/skills", body, 1,
		gin.Params{{Key: "id", Value: itoa(int(kp.ID))}})
	h.MountSkillToKP(c2)

	assertStatus(t, w2, http.StatusConflict)
	assertBodyContains(t, w2, "该技能已经挂载到此知识点")
}

// -- UnmountSkillFromKP Tests ---------------------------------

func TestUnmountSkillFromKP_InvalidKPID(t *testing.T) {
	h := newTestSkillHandler(t)

	w, c := newTestContextWithParams("DELETE", "/api/v1/knowledge-points/abc/skills/1", "", 1,
		gin.Params{{Key: "id", Value: "abc"}, {Key: "mount_id", Value: "1"}})
	h.UnmountSkillFromKP(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "无效的知识点 ID")
}

func TestUnmountSkillFromKP_InvalidMountID(t *testing.T) {
	h := newTestSkillHandler(t)

	w, c := newTestContextWithParams("DELETE", "/api/v1/knowledge-points/1/skills/abc", "", 1,
		gin.Params{{Key: "id", Value: "1"}, {Key: "mount_id", Value: "abc"}})
	h.UnmountSkillFromKP(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "无效的挂载 ID")
}

func TestUnmountSkillFromKP_MountNotFound(t *testing.T) {
	h := newTestSkillHandler(t)

	w, c := newTestContextWithParams("DELETE", "/api/v1/knowledge-points/1/skills/9999", "", 1,
		gin.Params{{Key: "id", Value: "1"}, {Key: "mount_id", Value: "9999"}})
	h.UnmountSkillFromKP(c)

	assertStatus(t, w, http.StatusNotFound)
	assertBodyContains(t, w, "挂载记录不存在")
}

func TestUnmountSkillFromKP_WrongKP(t *testing.T) {
	h := newTestSkillHandler(t)

	// Create a mount for KP 1
	mount := model.KPSkillMount{KPID: 1, SkillID: "test", ScaffoldLevel: model.ScaffoldHigh}
	h.DB.Create(&mount)

	// Try to unmount from KP 2
	w, c := newTestContextWithParams("DELETE", "/api/v1/knowledge-points/2/skills/"+itoa(int(mount.ID)), "", 1,
		gin.Params{{Key: "id", Value: "2"}, {Key: "mount_id", Value: itoa(int(mount.ID))}})
	h.UnmountSkillFromKP(c)

	assertStatus(t, w, http.StatusForbidden)
	assertBodyContains(t, w, "该挂载不属于此知识点")
}

func TestUnmountSkillFromKP_Success(t *testing.T) {
	h := newTestSkillHandler(t)

	mount := model.KPSkillMount{KPID: 1, SkillID: "test", ScaffoldLevel: model.ScaffoldHigh}
	h.DB.Create(&mount)

	w, c := newTestContextWithParams("DELETE", "/api/v1/knowledge-points/1/skills/"+itoa(int(mount.ID)), "", 1,
		gin.Params{{Key: "id", Value: "1"}, {Key: "mount_id", Value: itoa(int(mount.ID))}})
	h.UnmountSkillFromKP(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "技能已卸载")

	// Verify deleted from DB
	var count int64
	h.DB.Model(&model.KPSkillMount{}).Count(&count)
	if count != 0 {
		t.Errorf("mount count = %d, want 0", count)
	}
}

// -- UpdateKPSkillConfig Tests --------------------------------

func TestUpdateKPSkillConfig_InvalidKPID(t *testing.T) {
	h := newTestSkillHandler(t)

	body := `{"scaffold_level":"low"}`
	w, c := newTestContextWithParams("PATCH", "/api/v1/knowledge-points/abc/skills/1", body, 1,
		gin.Params{{Key: "id", Value: "abc"}, {Key: "mount_id", Value: "1"}})
	h.UpdateKPSkillConfig(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "无效的知识点 ID")
}

func TestUpdateKPSkillConfig_InvalidMountID(t *testing.T) {
	h := newTestSkillHandler(t)

	body := `{"scaffold_level":"low"}`
	w, c := newTestContextWithParams("PATCH", "/api/v1/knowledge-points/1/skills/abc", body, 1,
		gin.Params{{Key: "id", Value: "1"}, {Key: "mount_id", Value: "abc"}})
	h.UpdateKPSkillConfig(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "无效的挂载 ID")
}

func TestUpdateKPSkillConfig_InvalidJSON(t *testing.T) {
	h := newTestSkillHandler(t)

	w, c := newTestContextWithParams("PATCH", "/api/v1/knowledge-points/1/skills/1", "not json", 1,
		gin.Params{{Key: "id", Value: "1"}, {Key: "mount_id", Value: "1"}})
	h.UpdateKPSkillConfig(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "请求数据格式错误")
}

func TestUpdateKPSkillConfig_MountNotFound(t *testing.T) {
	h := newTestSkillHandler(t)

	body := `{"scaffold_level":"low"}`
	w, c := newTestContextWithParams("PATCH", "/api/v1/knowledge-points/1/skills/9999", body, 1,
		gin.Params{{Key: "id", Value: "1"}, {Key: "mount_id", Value: "9999"}})
	h.UpdateKPSkillConfig(c)

	assertStatus(t, w, http.StatusNotFound)
	assertBodyContains(t, w, "挂载记录不存在")
}

func TestUpdateKPSkillConfig_WrongKP(t *testing.T) {
	h := newTestSkillHandler(t)

	mount := model.KPSkillMount{KPID: 1, SkillID: "test", ScaffoldLevel: model.ScaffoldHigh}
	h.DB.Create(&mount)

	body := `{"scaffold_level":"low"}`
	w, c := newTestContextWithParams("PATCH", "/api/v1/knowledge-points/2/skills/"+itoa(int(mount.ID)), body, 1,
		gin.Params{{Key: "id", Value: "2"}, {Key: "mount_id", Value: itoa(int(mount.ID))}})
	h.UpdateKPSkillConfig(c)

	assertStatus(t, w, http.StatusForbidden)
	assertBodyContains(t, w, "该挂载不属于此知识点")
}

func TestUpdateKPSkillConfig_InvalidScaffoldLevel(t *testing.T) {
	h := newTestSkillHandler(t)

	mount := model.KPSkillMount{KPID: 1, SkillID: "test", ScaffoldLevel: model.ScaffoldHigh}
	h.DB.Create(&mount)

	body := `{"scaffold_level":"INVALID"}`
	w, c := newTestContextWithParams("PATCH", "/api/v1/knowledge-points/1/skills/"+itoa(int(mount.ID)), body, 1,
		gin.Params{{Key: "id", Value: "1"}, {Key: "mount_id", Value: itoa(int(mount.ID))}})
	h.UpdateKPSkillConfig(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "无效的支架等级")
}

func TestUpdateKPSkillConfig_NoUpdates(t *testing.T) {
	h := newTestSkillHandler(t)

	mount := model.KPSkillMount{KPID: 1, SkillID: "test", ScaffoldLevel: model.ScaffoldHigh}
	h.DB.Create(&mount)

	// Empty update
	body := `{}`
	w, c := newTestContextWithParams("PATCH", "/api/v1/knowledge-points/1/skills/"+itoa(int(mount.ID)), body, 1,
		gin.Params{{Key: "id", Value: "1"}, {Key: "mount_id", Value: itoa(int(mount.ID))}})
	h.UpdateKPSkillConfig(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "没有需要更新的字段")
}

func TestUpdateKPSkillConfig_UpdateScaffoldLevel(t *testing.T) {
	h := newTestSkillHandler(t)

	mount := model.KPSkillMount{KPID: 1, SkillID: "test", ScaffoldLevel: model.ScaffoldHigh}
	h.DB.Create(&mount)

	body := `{"scaffold_level":"low"}`
	w, c := newTestContextWithParams("PATCH", "/api/v1/knowledge-points/1/skills/"+itoa(int(mount.ID)), body, 1,
		gin.Params{{Key: "id", Value: "1"}, {Key: "mount_id", Value: itoa(int(mount.ID))}})
	h.UpdateKPSkillConfig(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "配置已更新")

	// Verify in DB
	var updated model.KPSkillMount
	h.DB.First(&updated, mount.ID)
	if updated.ScaffoldLevel != model.ScaffoldLow {
		t.Errorf("ScaffoldLevel = %q, want %q", updated.ScaffoldLevel, model.ScaffoldLow)
	}
}

func TestUpdateKPSkillConfig_UpdateProgressiveRule(t *testing.T) {
	h := newTestSkillHandler(t)

	mount := model.KPSkillMount{KPID: 1, SkillID: "test", ScaffoldLevel: model.ScaffoldHigh}
	h.DB.Create(&mount)

	body := `{"progressive_rule":{"threshold":0.8}}`
	w, c := newTestContextWithParams("PATCH", "/api/v1/knowledge-points/1/skills/"+itoa(int(mount.ID)), body, 1,
		gin.Params{{Key: "id", Value: "1"}, {Key: "mount_id", Value: itoa(int(mount.ID))}})
	h.UpdateKPSkillConfig(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "配置已更新")
}
