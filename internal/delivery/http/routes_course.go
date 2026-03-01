package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// registerCourseRoutes sets up course, skill store, chapter mounting,
// custom skill CRUD, and knowledge point enrichment endpoints.
// All require TEACHER role or above.
func registerCourseRoutes(
	protected *gin.RouterGroup,
	db *gorm.DB,
	courseHandler *handler.CourseHandler,
	skillHandler *handler.SkillHandler,
	customSkillHandler *handler.CustomSkillHandler,
	kgHandler *handler.KnowledgeGraphHandler,
) {
	teacherRoles := middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin)

	// Course management
	courses := protected.Group("/courses")
	courses.Use(teacherRoles)
	{
		courses.GET("", courseHandler.ListCourses)
		courses.POST("", courseHandler.CreateCourse)
		courses.POST("/:id/materials", courseHandler.UploadMaterial)
		courses.GET("/:id/outline", courseHandler.GetOutline)
		courses.GET("/:id/documents", courseHandler.GetDocumentStatus)
		courses.POST("/:id/search", courseHandler.SearchCourse)
		courses.DELETE("/:id/documents/:doc_id", courseHandler.DeleteDocument)
		courses.POST("/:id/documents/:doc_id/retry", courseHandler.RetryDocument)
	}

	// Skill Store — Phase 3
	skills := protected.Group("/skills")
	skills.Use(teacherRoles)
	{
		skills.GET("", skillHandler.ListSkills)
		skills.GET("/:id", skillHandler.GetSkillDetail)
	}

	// Skill Mounting — Phase 3
	chapters := protected.Group("/chapters")
	chapters.Use(teacherRoles)
	{
		chapters.POST("/:id/skills", skillHandler.MountSkill)
		chapters.PATCH("/:id/skills/:mount_id", skillHandler.UpdateSkillConfig)
		chapters.DELETE("/:id/skills/:mount_id", skillHandler.UnmountSkill)
	}

	// Custom Skill CRUD — Phase 4 / §6.4
	customSkills := protected.Group("/custom-skills")
	customSkills.Use(teacherRoles)
	{
		customSkills.POST("", customSkillHandler.CreateCustomSkill)
		customSkills.GET("", customSkillHandler.ListCustomSkills)
		customSkills.GET("/:id", customSkillHandler.GetCustomSkill)
		customSkills.PUT("/:id", customSkillHandler.UpdateCustomSkill)
		customSkills.DELETE("/:id", customSkillHandler.DeleteCustomSkill)
		customSkills.POST("/:id/publish", customSkillHandler.PublishCustomSkill)
		customSkills.POST("/:id/share", customSkillHandler.ShareCustomSkill)
		customSkills.POST("/:id/archive", customSkillHandler.ArchiveCustomSkill)
		customSkills.GET("/:id/versions", customSkillHandler.ListVersions)
	}

	// Knowledge Graph Enrichment — Phase B
	kps := protected.Group("/knowledge-points")
	kps.Use(teacherRoles)
	{
		// Misconception CRUD
		kps.POST("/:id/misconceptions", kgHandler.CreateMisconception)
		kps.GET("/:id/misconceptions", kgHandler.ListMisconceptions)
		kps.DELETE("/:id/misconceptions/:misconception_id", kgHandler.DeleteMisconception)

		// Cross-Disciplinary Links
		kps.POST("/:id/cross-links", kgHandler.CreateCrossLink)
		kps.GET("/:id/cross-links", kgHandler.ListCrossLinks)
		kps.DELETE("/:id/cross-links/:link_id", kgHandler.DeleteCrossLink)

		// Prerequisite Management
		kps.POST("/:id/prerequisites", kgHandler.CreatePrerequisite)
		kps.GET("/:id/prerequisites", kgHandler.GetPrerequisites)
	}
}
