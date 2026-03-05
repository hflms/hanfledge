package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
)

// MountSkillToKP mounts a skill directly to a specific knowledge point.
// POST /api/v1/knowledge-points/:id/skills
func (h *SkillHandler) MountSkillToKP(c *gin.Context) {
	kpID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识点 ID"})
		return
	}

	var req MountSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	if _, ok := h.Registry.GetSkill(req.SkillID); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "技能不存在: " + req.SkillID})
		return
	}

	var kp model.KnowledgePoint
	if err := h.DB.First(&kp, kpID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "知识点不存在"})
		return
	}

	if req.ScaffoldLevel == "" {
		req.ScaffoldLevel = model.ScaffoldHigh
	}

	var existing model.KPSkillMount
	if err := h.DB.Where("kp_id = ? AND skill_id = ?", kp.ID, req.SkillID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该技能已经挂载到此知识点"})
		return
	}

	constraintsJSON := "{}"
	if req.ConstraintsJSON != nil {
		if data, err := marshalJSON(req.ConstraintsJSON); err == nil {
			constraintsJSON = string(data)
		}
	}

	var progressiveRule *string
	if req.ProgressiveRule != nil {
		if data, err := marshalJSON(req.ProgressiveRule); err == nil {
			s := string(data)
			progressiveRule = &s
		}
	}

	mount := model.KPSkillMount{
		KPID:            kp.ID,
		SkillID:         req.SkillID,
		ScaffoldLevel:   req.ScaffoldLevel,
		ConstraintsJSON: constraintsJSON,
		Priority:        req.Priority,
		ProgressiveRule: progressiveRule,
	}

	if err := h.DB.Create(&mount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "挂载技能失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "技能挂载成功",
		"count":   1,
		"mounts":  []model.KPSkillMount{mount},
	})
}

// UnmountSkillFromKP removes a skill mount from a specific knowledge point.
// DELETE /api/v1/knowledge-points/:id/skills/:mount_id
func (h *SkillHandler) UnmountSkillFromKP(c *gin.Context) {
	kpID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识点 ID"})
		return
	}

	mountID, err := strconv.ParseUint(c.Param("mount_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的挂载 ID"})
		return
	}

	var mount model.KPSkillMount
	if err := h.DB.First(&mount, mountID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "挂载记录不存在"})
		return
	}

	if mount.KPID != uint(kpID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "该挂载不属于此知识点"})
		return
	}

	if err := h.DB.Delete(&mount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "卸载技能失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "技能已卸载"})
}

// UpdateKPSkillConfig updates the scaffold level and progressive rule of a specific KP skill mount.
// PATCH /api/v1/knowledge-points/:id/skills/:mount_id
func (h *SkillHandler) UpdateKPSkillConfig(c *gin.Context) {
	kpID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识点 ID"})
		return
	}

	mountID, err := strconv.ParseUint(c.Param("mount_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的挂载 ID"})
		return
	}

	var req UpdateSkillConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	var mount model.KPSkillMount
	if err := h.DB.First(&mount, mountID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "挂载记录不存在"})
		return
	}

	if mount.KPID != uint(kpID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "该挂载不属于此知识点"})
		return
	}

	updates := map[string]interface{}{}

	if req.ScaffoldLevel != nil {
		level := *req.ScaffoldLevel
		if level != model.ScaffoldHigh && level != model.ScaffoldMedium && level != model.ScaffoldLow {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的支架等级，可选: high, medium, low"})
			return
		}
		updates["scaffold_level"] = level
	}

	if req.ProgressiveRule != nil {
		data, err := marshalJSON(req.ProgressiveRule)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "渐进规则格式错误"})
			return
		}
		s := string(data)
		updates["progressive_rule"] = s
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有需要更新的字段"})
		return
	}

	if err := h.DB.Model(&mount).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败: " + err.Error()})
		return
	}

	h.DB.First(&mount, mountID)

	c.JSON(http.StatusOK, gin.H{
		"message": "配置已更新",
		"mount":   mount,
	})
}
