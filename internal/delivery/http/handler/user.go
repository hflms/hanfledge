package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserHandler handles user management requests.
type UserHandler struct {
	DB *gorm.DB
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{DB: db}
}

// ── School CRUD ─────────────────────────────────────────────

// ListSchools returns all schools.
// GET /api/v1/schools
func (h *UserHandler) ListSchools(c *gin.Context) {
	var schools []model.School
	if err := h.DB.Find(&schools).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询学校列表失败"})
		return
	}
	c.JSON(http.StatusOK, schools)
}

// CreateSchool creates a new school.
// POST /api/v1/schools
func (h *UserHandler) CreateSchool(c *gin.Context) {
	var school model.School
	if err := c.ShouldBindJSON(&school); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误"})
		return
	}
	if err := h.DB.Create(&school).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建学校失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, school)
}

// ── Class CRUD ──────────────────────────────────────────────

// ListClasses returns all classes, optionally filtered by school_id.
// GET /api/v1/classes?school_id=X
func (h *UserHandler) ListClasses(c *gin.Context) {
	var classes []model.Class
	query := h.DB.Preload("School")
	if schoolID := c.Query("school_id"); schoolID != "" {
		query = query.Where("school_id = ?", schoolID)
	}
	if err := query.Find(&classes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询班级列表失败"})
		return
	}
	c.JSON(http.StatusOK, classes)
}

// CreateClass creates a new class under a school.
// POST /api/v1/classes
func (h *UserHandler) CreateClass(c *gin.Context) {
	var class model.Class
	if err := c.ShouldBindJSON(&class); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误"})
		return
	}
	if err := h.DB.Create(&class).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建班级失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, class)
}

// ── User Management ─────────────────────────────────────────

// ListUsers returns all users, optionally filtered by school_id.
// GET /api/v1/users?school_id=X
func (h *UserHandler) ListUsers(c *gin.Context) {
	var users []model.User
	query := h.DB.Preload("SchoolRoles.Role").Preload("SchoolRoles.School")

	if schoolID := c.Query("school_id"); schoolID != "" {
		sid, _ := strconv.ParseUint(schoolID, 10, 32)
		// Find users who have a role in the specified school
		var userIDs []uint
		h.DB.Model(&model.UserSchoolRole{}).
			Where("school_id = ?", uint(sid)).
			Pluck("user_id", &userIDs)
		query = query.Where("id IN ?", userIDs)
	}

	if err := query.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户列表失败"})
		return
	}
	c.JSON(http.StatusOK, users)
}

// CreateUserRequest represents the request body for creating a user.
type CreateUserRequest struct {
	Phone       string         `json:"phone" binding:"required"`
	Password    string         `json:"password" binding:"required,min=6"`
	DisplayName string         `json:"display_name" binding:"required"`
	Email       *string        `json:"email,omitempty"`
	SchoolID    *uint          `json:"school_id,omitempty"`
	RoleName    model.RoleName `json:"role" binding:"required"`
}

// CreateUser creates a new user with a role assignment.
// POST /api/v1/users
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	// Find role
	var role model.Role
	if err := h.DB.Where("name = ?", req.RoleName).First(&role).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色不存在: " + string(req.RoleName)})
		return
	}

	// Create user within a transaction
	user := model.User{
		Phone:        req.Phone,
		PasswordHash: string(hash),
		DisplayName:  req.DisplayName,
		Email:        req.Email,
		Status:       model.UserStatusActive,
	}

	err = h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}

		// Assign role
		roleAssignment := model.UserSchoolRole{
			UserID:   user.ID,
			SchoolID: req.SchoolID,
			RoleID:   role.ID,
		}
		return tx.Create(&roleAssignment).Error
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败: " + err.Error()})
		return
	}

	// Reload with roles
	h.DB.Preload("SchoolRoles.Role").First(&user, user.ID)
	c.JSON(http.StatusCreated, user)
}

// BatchCreateRequest represents the request for batch user creation.
type BatchCreateRequest struct {
	Users []CreateUserRequest `json:"users" binding:"required,min=1"`
}

// BatchCreateUsers creates multiple users at once.
// POST /api/v1/users/batch
func (h *UserHandler) BatchCreateUsers(c *gin.Context) {
	var req BatchCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	var created []model.User
	var errors []string

	for i, u := range req.Users {
		hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			errors = append(errors, "用户 "+strconv.Itoa(i)+": 密码加密失败")
			continue
		}

		var role model.Role
		if err := h.DB.Where("name = ?", u.RoleName).First(&role).Error; err != nil {
			errors = append(errors, "用户 "+u.Phone+": 角色不存在")
			continue
		}

		user := model.User{
			Phone:        u.Phone,
			PasswordHash: string(hash),
			DisplayName:  u.DisplayName,
			Email:        u.Email,
			Status:       model.UserStatusActive,
		}

		err = h.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
			return tx.Create(&model.UserSchoolRole{
				UserID:   user.ID,
				SchoolID: u.SchoolID,
				RoleID:   role.ID,
			}).Error
		})

		if err != nil {
			errors = append(errors, "用户 "+u.Phone+": "+err.Error())
			continue
		}
		created = append(created, user)
	}

	c.JSON(http.StatusOK, gin.H{
		"created_count": len(created),
		"error_count":   len(errors),
		"errors":        errors,
		"users":         created,
	})
}
