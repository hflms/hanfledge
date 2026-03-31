package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// registerAnalyticsRoutes sets up dashboard analytics, session analytics,
// and data export endpoints. All require TEACHER role or above.
func registerAnalyticsRoutes(
	protected *gin.RouterGroup,
	db *gorm.DB,
	dashboardHandler *handler.DashboardHandler,
	analyticsHandler *handler.AnalyticsHandler,
	exportHandler *handler.ExportHandler,
) {
	teacherRoles := middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin)

	// Dashboard Analytics — Phase 5 + Phase G
	dashboard := protected.Group("/dashboard")
	dashboard.Use(teacherRoles)
	{
		dashboard.GET("/knowledge-radar", dashboardHandler.GetKnowledgeRadar)
		dashboard.GET("/skill-effectiveness", analyticsHandler.GetSkillEffectiveness) // Phase G
		dashboard.GET("/live-monitor", dashboardHandler.GetLiveMonitor)               // 实时监控概览
		dashboard.GET("/activities/:id/live", dashboardHandler.GetActivityLiveDetail) // 活动实时详情
	}

	// Student Mastery (TEACHER) — Phase 5
	students := protected.Group("/students")
	students.Use(teacherRoles)
	{
		students.GET("/:id/mastery", dashboardHandler.GetStudentMastery)
	}

	// Session Analytics (TEACHER) — Phase G
	sessionAnalytics := protected.Group("/sessions")
	sessionAnalytics.Use(teacherRoles)
	{
		sessionAnalytics.GET("/:id/inquiry-tree", analyticsHandler.GetInquiryTree)
		sessionAnalytics.GET("/:id/interactions", analyticsHandler.GetInteractionLog)
	}

	// Data Export (TEACHER) — CSV Downloads
	export := protected.Group("/export")
	export.Use(teacherRoles)
	{
		export.GET("/activities/:id/sessions", exportHandler.ExportActivitySessions)
		export.GET("/courses/:id/mastery", exportHandler.ExportClassMastery)
		export.GET("/courses/:id/error-notebook", exportHandler.ExportErrorNotebook)
		export.GET("/sessions/:id/interactions", exportHandler.ExportInteractionLog)
	}

	// Performance Monitoring (ANY authenticated user)
	analytics := protected.Group("/analytics")
	{
		analytics.POST("/performance", analyticsHandler.RecordPerformance)
	}
}
