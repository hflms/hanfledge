package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// registerActivityRoutes sets up learning activity and session endpoints.
func registerActivityRoutes(
	protected *gin.RouterGroup,
	db *gorm.DB,
	activityHandler *handler.ActivityHandler,
	sessionHandler *handler.SessionHandler,
	dashboardHandler *handler.DashboardHandler,
) {
	teacherRoles := middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin)

	// Instructional Designers (TEACHER)
	protected.GET("/designers", teacherRoles, activityHandler.ListDesigners)

	// Learning Activities (TEACHER) — Phase 4
	activities := protected.Group("/activities")
	activities.Use(teacherRoles)
	{
		activities.POST("", activityHandler.CreateActivity)
		activities.GET("", activityHandler.ListActivities)
		activities.GET("/:id", activityHandler.GetActivity)                       // Activity detail
		activities.PUT("/:id", activityHandler.UpdateActivity)                    // Update draft activity
		activities.PUT("/:id/steps", activityHandler.SaveSteps)                   // Batch save steps
		activities.POST("/:id/upload", activityHandler.UploadAsset)               // Upload activity asset
		activities.POST("/:id/steps/suggest", activityHandler.SuggestStepContent) // AI suggest step content
		activities.POST("/:id/publish", activityHandler.PublishActivity)
		activities.POST("/:id/preview", activityHandler.PreviewActivity)      // Sandbox preview
		activities.GET("/:id/sessions", dashboardHandler.GetActivitySessions) // Phase 5
	}

	// Activity Join & Sessions (any authenticated user)
	protected.POST("/activities/:id/join", activityHandler.JoinActivity)
	protected.GET("/sessions/:id", activityHandler.GetSession)
	protected.PUT("/sessions/:id/step", activityHandler.UpdateSessionStep)
	protected.POST("/sessions/:id/next-step", activityHandler.NextGuidedStep)

	// WebSocket Session Stream — Phase 4
	protected.GET("/sessions/:id/stream", sessionHandler.StreamSession)
	// Teacher Intervention - Phase 6
	protected.POST("/sessions/:id/intervention", teacherRoles, sessionHandler.HandleIntervention)
}
