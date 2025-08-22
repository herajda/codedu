package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ensureClassMember(c *gin.Context, classID uuid.UUID) bool {
	uid := getUserID(c)
	role := c.GetString("role")
	if role == "admin" {
		return true
	}
	var ok bool
	var err error
	if role == "teacher" {
		ok, err = IsTeacherOfClass(classID, uid)
	} else {
		ok, err = IsStudentOfClass(classID, uid)
	}
	if err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return false
	}
	return true
}

func createForumMessageHandler(c *gin.Context) {
	cid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if !ensureClassMember(c, cid) {
		return
	}
	var req struct {
		Text     string  `json:"text"`
		Image    *string `json:"image"`
		FileName *string `json:"file_name"`
		File     *string `json:"file"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Text) == "" && (req.Image == nil || *req.Image == "") && (req.File == nil || *req.File == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty message"})
		return
	}
	if req.Image != nil && len(*req.Image) > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image too large"})
		return
	}
	if req.File != nil && len(*req.File) > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large"})
		return
	}
	m := &ForumMessage{ClassID: cid, UserID: getUserID(c), Text: req.Text, Image: req.Image, FileName: req.FileName, File: req.File}
	if err := CreateForumMessage(m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusCreated, m)
}

func listForumMessagesHandler(c *gin.Context) {
	cid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if !ensureClassMember(c, cid) {
		return
	}
	limit := 50
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	offset := 0
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}
	msgs, err := ListForumMessages(cid, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, msgs)
}
