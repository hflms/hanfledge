package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// registerWeKnoraRoutes sets up WeKnora knowledge base integration endpoints.
// Only registered when WeKnora integration is enabled.
// All require TEACHER role or above.
func registerWeKnoraRoutes(
	protected *gin.RouterGroup,
	db *gorm.DB,
	wkHandler *handler.WeKnoraHandler,
) {
	teacherRoles := middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin)

	// WeKnora proxy endpoints — browse remote knowledge bases
	wk := protected.Group("/weknora")
	wk.Use(teacherRoles)
	{
		wk.POST("/knowledge-bases", wkHandler.CreateKnowledgeBase)
		wk.GET("/knowledge-bases", wkHandler.ListKnowledgeBases)
		wk.GET("/knowledge-bases/:kb_id", wkHandler.GetKnowledgeBase)
		wk.GET("/knowledge-bases/:kb_id/knowledge", wkHandler.ListKnowledge)
		wk.DELETE("/knowledge-bases/:kb_id", wkHandler.DeleteKnowledgeBase)
	}
}

// registerWeKnoraCourseRoutes sets up course-level WeKnora binding endpoints.
// These are mounted under /courses/:id/ to manage which WeKnora KBs are
// referenced by a course and to perform retrieval within them.
func registerWeKnoraCourseRoutes(
	protected *gin.RouterGroup,
	db *gorm.DB,
	wkHandler *handler.WeKnoraHandler,
) {
	teacherRoles := middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin)

	courses := protected.Group("/courses")
	courses.Use(teacherRoles)
	{
		courses.POST("/:id/weknora-refs", wkHandler.BindKnowledgeBase)
		courses.GET("/:id/weknora-refs", wkHandler.ListBoundKnowledgeBases)
		courses.DELETE("/:id/weknora-refs/:ref_id", wkHandler.UnbindKnowledgeBase)
		courses.POST("/:id/weknora-search", wkHandler.SearchKnowledgeBase)
	}
}
