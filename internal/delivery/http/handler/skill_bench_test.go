package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/plugin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func BenchmarkMountSkill(b *testing.B) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Fatal(err)
	}

	err = db.AutoMigrate(&model.Course{}, &model.Chapter{}, &model.KnowledgePoint{}, &model.KPSkillMount{})
	if err != nil {
		b.Fatal(err)
	}

	course := model.Course{Title: "Test Course"}
	db.Create(&course)
	chapter := model.Chapter{CourseID: course.ID, Title: "Test Chapter"}
	db.Create(&chapter)

	// create many KPs
	for i := 0; i < 50; i++ {
		db.Create(&model.KnowledgePoint{ChapterID: chapter.ID, Title: fmt.Sprintf("KP %d", i)})
	}

	gin.SetMode(gin.TestMode)
    registry := plugin.NewRegistry()
    registry.RegisterSkillWithMetadata(plugin.SkillMetadata{
		ID:      "test-skill",
		Name:    "Test Skill test-skill",
		Version: "1.0.0",
	})
	handler := NewSkillHandler(db, registry, nil)

	reqData := MountSkillRequest{
		SkillID:       "test-skill",
		ScaffoldLevel: model.ScaffoldHigh,
	}
	body, _ := json.Marshal(reqData)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// We need to truncate the mount table each time so we insert new mounts
		db.Exec("DELETE FROM kp_skill_mounts")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/chapters/%d/skills", chapter.ID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", chapter.ID)}}

		handler.MountSkill(c)

		if w.Code != http.StatusCreated && w.Code != http.StatusOK {
			b.Fatalf("expected status 201 or 200, got %d. body: %s", w.Code, w.Body.String())
		}
	}
}
