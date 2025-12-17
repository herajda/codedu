package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Helper to get a setting string, defaulting to defaultValue if missing/error
func GetSystemSetting(key, defaultValue string) string {
	var val string
	err := DB.Get(&val, "SELECT value FROM system_settings WHERE key=$1", key)
	if err != nil {
		return defaultValue
	}
	return val
}

func SetSystemSetting(key, value string) error {
	_, err := DB.Exec(`
		INSERT INTO system_settings (key, value)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, value,
	)
	return err
}

// Handler: GET /api/admin/system-settings
func getSystemSettings(c *gin.Context) {
	// For now we only have one setting, but we can return a map
	forceBakalariEmail := GetSystemSetting("force_bakalari_email", "true")
	c.JSON(http.StatusOK, gin.H{
		"force_bakalari_email": forceBakalariEmail == "true",
	})
}

// Handler: PUT /api/admin/system-settings
func updateSystemSettings(c *gin.Context) {
	var req struct {
		ForceBakalariEmail *bool `json:"force_bakalari_email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ForceBakalariEmail != nil {
		val := "false"
		if *req.ForceBakalariEmail {
			val = "true"
		}
		if err := SetSystemSetting("force_bakalari_email", val); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
			return
		}
	}
	c.Status(http.StatusNoContent)
}
