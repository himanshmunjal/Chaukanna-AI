package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"chaukanna/internal/hub"
	"chaukanna/internal/models"
)

// gormDB is a short alias used in Preload callbacks.
type gormDB = gorm.DB

// ---------------------------------------------------------------------------
// Calls
// ---------------------------------------------------------------------------

// GET /api/calls?limit=50
func (s *Server) listCalls(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var calls []models.Call
	if err := s.ownCalls(c).Preload("ProtectedUser").Order("id desc").Limit(limit).Find(&calls).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, calls)
}

// GET /api/calls/active
func (s *Server) activeCalls(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ids": s.sessions.ActiveCallIDs(c.GetString(hub.OwnerKey))})
}

// GET /api/calls/:id
func (s *Server) getCall(c *gin.Context) {
	var call models.Call
	id, ok := pathID(c)
	if !ok {
		return
	}
	err := s.ownCalls(c).
		Preload("ProtectedUser").
		Preload("Transcript", func(db *gormDB) *gormDB { return db.Order("time_sec, id") }).
		Preload("Flags", func(db *gormDB) *gormDB { return db.Order("time_sec, id") }).
		Preload("Alerts").
		First(&call, id).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "call not found"})
		return
	}
	c.JSON(http.StatusOK, call)
}

// GET /api/stats
func (s *Server) stats(c *gin.Context) {
	var total, scams, warned, alerts int64
	s.ownCalls(c).Model(&models.Call{}).Count(&total)
	s.ownCalls(c).Model(&models.Call{}).Where("state IN ?", []string{"scam", "warned"}).Count(&scams)
	s.ownCalls(c).Model(&models.Call{}).Where("warned_at IS NOT NULL").Count(&warned)
	s.db.Model(&models.Alert{}).Where("status IN ?", []string{"sent", "logged"}).
		Where("call_id IN (?)", s.ownCalls(c).Model(&models.Call{}).Select("id")).Count(&alerts)

	var labelledScam, labelledLegit int64
	s.ownCalls(c).Model(&models.Call{}).Where("test_label = ?", "scam").Count(&labelledScam)
	s.ownCalls(c).Model(&models.Call{}).Where("test_label = ?", "legit").Count(&labelledLegit)

	var avg struct{ Value *float64 }
	s.ownCalls(c).Model(&models.Call{}).Select("AVG(detected_after_sec) AS value").
		Where("detected_after_sec IS NOT NULL").Scan(&avg)

	c.JSON(http.StatusOK, gin.H{
		"total_calls":       total,
		"scams_detected":    scams,
		"warnings_played":   warned,
		"alerts_sent":       alerts,
		"avg_detection_sec": avg.Value,
		"labelled_scam":     labelledScam,
		"labelled_legit":    labelledLegit,
		"active_calls":      len(s.sessions.ActiveCallIDs(c.GetString(hub.OwnerKey))),
	})
}

// ---------------------------------------------------------------------------
// Protected users
// ---------------------------------------------------------------------------

// GET /api/users
func (s *Server) listUsers(c *gin.Context) {
	var users []models.ProtectedUser
	s.db.Order("id").Find(&users)
	c.JSON(http.StatusOK, users)
}

type createUserRequest struct {
	Name        string `json:"name" binding:"required"`
	Phone       string `json:"phone" binding:"required"`
	Language    string `json:"language"`
	FamilyName  string `json:"family_name"`
	FamilyPhone string `json:"family_phone"`
	Consent     bool   `json:"consent"` // the person agreed to have their calls protected
}

// POST /api/users
func (s *Server) createUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and phone are required"})
		return
	}
	user := models.ProtectedUser{
		Name:        strings.TrimSpace(req.Name),
		Phone:       models.NormalizePhone(req.Phone),
		Language:    strings.TrimSpace(req.Language),
		FamilyName:  strings.TrimSpace(req.FamilyName),
		FamilyPhone: models.NormalizePhone(req.FamilyPhone),
	}
	if user.Language == "" {
		user.Language = "hi"
	}
	if req.Consent {
		now := time.Now()
		user.ConsentAt = &now
	}
	if err := s.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "could not save user (is the phone already registered?)"})
		return
	}
	c.JSON(http.StatusCreated, user)
}

// DELETE /api/users/:id
func (s *Server) deleteUser(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := s.db.Delete(&models.ProtectedUser{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Scam numbers
// ---------------------------------------------------------------------------

// GET /api/scam-numbers
func (s *Server) listScamNumbers(c *gin.Context) {
	var numbers []models.ScamNumber
	s.db.Order("updated_at desc").Find(&numbers)
	c.JSON(http.StatusOK, numbers)
}

type reportNumberRequest struct {
	Phone  string `json:"phone" binding:"required"`
	Reason string `json:"reason"`
}

// POST /api/scam-numbers  (the family reports a number by hand)
func (s *Server) reportScamNumber(c *gin.Context) {
	var req reportNumberRequest
	if err := c.ShouldBindJSON(&req); err != nil || models.NormalizePhone(req.Phone) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "phone is required"})
		return
	}
	phone := models.NormalizePhone(req.Phone)
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "Reported by family"
	}

	var n models.ScamNumber
	s.db.Where(models.ScamNumber{Phone: phone}).FirstOrInit(&n)
	if n.ID == 0 {
		n.Source = "manual"
	}
	n.Reports++
	n.LastReason = reason
	if err := s.db.Save(&n).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, n)
}

// DELETE /api/scam-numbers/:id  (unblock, e.g. after a false alarm)
func (s *Server) deleteScamNumber(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := s.db.Delete(&models.ScamNumber{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
