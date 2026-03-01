package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// registerStudentRoutes sets up student-facing endpoints.
func registerStudentRoutes(
	protected *gin.RouterGroup,
	db *gorm.DB,
	activityHandler *handler.ActivityHandler,
	dashboardHandler *handler.DashboardHandler,
	kgHandler *handler.KnowledgeGraphHandler,
	achievementHandler *handler.AchievementHandler,
) {
	student := protected.Group("/student")
	student.Use(middleware.RBAC(db, model.RoleStudent, model.RoleSysAdmin))
	{
		student.GET("/activities", activityHandler.StudentListActivities)
		student.GET("/mastery", dashboardHandler.GetSelfMastery)                     // Phase 5
		student.GET("/knowledge-map", kgHandler.GetStudentKnowledgeMap)              // Knowledge Map
		student.GET("/error-notebook", dashboardHandler.GetErrorNotebook)            // Error Notebook
		student.GET("/achievements", achievementHandler.GetMyAchievements)           // Achievements
		student.GET("/achievements/definitions", achievementHandler.ListDefinitions) // Achievement defs
	}
}
