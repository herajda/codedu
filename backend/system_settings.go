package main

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

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

const systemVariablePrefix = "var."

var systemVariableKeyRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{1,63}$`)

func normalizeSystemVariableKey(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if strings.HasPrefix(key, systemVariablePrefix) {
		key = strings.TrimPrefix(key, systemVariablePrefix)
	}
	if key == "" {
		return "", errors.New("key is required")
	}
	if len(key) > 64 {
		return "", errors.New("key is too long")
	}
	if !systemVariableKeyRe.MatchString(key) {
		return "", errors.New("key must start with a letter and contain only letters, numbers, and underscores")
	}
	return key, nil
}

func GetSystemVariable(key, defaultValue string) string {
	normalized, err := normalizeSystemVariableKey(key)
	if err != nil {
		return defaultValue
	}
	return GetSystemSetting(systemVariablePrefix+normalized, defaultValue)
}

// Handler: GET /api/admin/system-settings
func getSystemSettings(c *gin.Context) {
	forceBakalariEmail := GetSystemSetting("force_bakalari_email", "true")
	allowMicrosoftLogin := GetSystemSetting("allow_microsoft_login", "true")
	allowBakalariLogin := GetSystemSetting("allow_bakalari_login", "true")
	c.JSON(http.StatusOK, gin.H{
		"force_bakalari_email":  forceBakalariEmail == "true",
		"allow_microsoft_login": allowMicrosoftLogin == "true",
		"allow_bakalari_login":  allowBakalariLogin == "true",
	})
}

// Handler: PUT /api/admin/system-settings
func updateSystemSettings(c *gin.Context) {
	var req struct {
		ForceBakalariEmail  *bool `json:"force_bakalari_email"`
		AllowMicrosoftLogin *bool `json:"allow_microsoft_login"`
		AllowBakalariLogin  *bool `json:"allow_bakalari_login"`
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

	if req.AllowMicrosoftLogin != nil {
		val := "false"
		if *req.AllowMicrosoftLogin {
			val = "true"
		}
		if err := SetSystemSetting("allow_microsoft_login", val); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
			return
		}
	}

	if req.AllowBakalariLogin != nil {
		val := "false"
		if *req.AllowBakalariLogin {
			val = "true"
		}
		if err := SetSystemSetting("allow_bakalari_login", val); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

type systemVariable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type systemVariableRow struct {
	Key   string `db:"key"`
	Value string `db:"value"`
}

// Handler: GET /api/admin/system-variables
func listSystemVariables(c *gin.Context) {
	rows := []systemVariableRow{}
	if err := DB.Select(&rows, "SELECT key, value FROM system_settings WHERE key LIKE $1 ORDER BY key", systemVariablePrefix+"%"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	vars := make([]systemVariable, 0, len(rows))
	for _, row := range rows {
		key := strings.TrimPrefix(row.Key, systemVariablePrefix)
		if key == "" {
			continue
		}
		vars = append(vars, systemVariable{Key: key, Value: row.Value})
	}
	c.JSON(http.StatusOK, vars)
}

// Handler: POST /api/admin/system-variables
func upsertSystemVariable(c *gin.Context) {
	var req systemVariable
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	key, err := normalizeSystemVariableKey(req.Key)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Value) > 4096 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value is too long"})
		return
	}
	if err := SetSystemSetting(systemVariablePrefix+key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

// Handler: DELETE /api/admin/system-variables/:key
func deleteSystemVariable(c *gin.Context) {
	key, err := normalizeSystemVariableKey(c.Param("key"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := DB.Exec("DELETE FROM system_settings WHERE key=$1", systemVariablePrefix+key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}
