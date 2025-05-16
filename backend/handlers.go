package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func deleteUser(c *gin.Context) {
	c.JSON(501, gin.H{"error": "not implemented"})
}

func listSubs(c *gin.Context) {
	c.JSON(501, gin.H{"error": "not implemented"})
}

// createAssignment: POST /api/assignments
func createAssignment(c *gin.Context) {
	var req struct {
		Title       string    `json:"title" binding:"required"`
		Description string    `json:"description" binding:"required"`
		Deadline    time.Time `json:"deadline" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	a := &Assignment{
		Title:       req.Title,
		Description: req.Description,
		Deadline:    req.Deadline,
		CreatedBy:   c.GetInt("userID"),
	}
	if err := CreateAssignment(a); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create assignment"})
		return
	}
	c.JSON(http.StatusCreated, a)
}

// listAssignments: GET /api/assignments
func listAssignments(c *gin.Context) {
	list, err := ListAssignments()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list"})
		return
	}
	c.JSON(http.StatusOK, list)
}

// getAssignment: GET /api/assignments/:id
func getAssignment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	a, err := GetAssignment(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, a)
}

// updateAssignment: PUT /api/assignments/:id
func updateAssignment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Title       string    `json:"title" binding:"required"`
		Description string    `json:"description" binding:"required"`
		Deadline    time.Time `json:"deadline" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	a := &Assignment{
		ID:          id,
		Title:       req.Title,
		Description: req.Description,
		Deadline:    req.Deadline,
	}
	if err := UpdateAssignment(a); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update"})
		return
	}
	c.JSON(http.StatusOK, a)
}

// deleteAssignment: DELETE /api/assignments/:id
func deleteAssignment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := DeleteAssignment(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete"})
		return
	}
	c.Status(http.StatusNoContent)
}
