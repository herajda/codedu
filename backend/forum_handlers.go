package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func ensureClassMember(c *gin.Context, classID int) bool {
	uid := c.GetInt("userID")
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
	cid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if !ensureClassMember(c, cid) {
		return
	}
	var req struct {
		Text  string  `json:"text"`
		Image *string `json:"image"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Text) == "" && (req.Image == nil || *req.Image == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty message"})
		return
	}
	if req.Image != nil && len(*req.Image) > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image too large"})
		return
	}
	m := &ForumMessage{ClassID: cid, UserID: c.GetInt("userID"), Text: req.Text, Image: req.Image}
	if err := CreateForumMessage(m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusCreated, m)
}

func listForumMessagesHandler(c *gin.Context) {
	cid, err := strconv.Atoi(c.Param("id"))
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
