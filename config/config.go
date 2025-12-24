package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

// Config 应用程序配置结构
type Config struct {
	// 服务器配置
	Port  int  `json:"port"`
	Debug bool `json:"debug"`

	// API配置
	APIKey             string `json:"api_key"`
	Models             string `json:"models"`
	SystemPromptInject string `json:"system_prompt_inject"`
	Timeout            int    `json:"timeout"`
	MaxInputLength     int    `json:"max_input_length"`

	// Cursor IDE 客户端配置
	CursorAPIURL     string `json:"cursor_api_url"`
	CursorToken      string `json:"cursor_token"`       // Cursor JWT Token (从 IDE 获取)
	CursorClientKey  string `json:"cursor_client_key"`  // x-client-key
	CursorChecksum   string `json:"cursor_checksum"`    // x-cursor-checksum
	CursorVersion    string `json:"cursor_version"`     // x-cursor-client-version
	CursorTimezone   string `json:"cursor_timezone"`    // x-cursor-timezone
	CursorGhostMode  bool   `json:"cursor_ghost_mode"`  // x-ghost-mode
	CursorWorkingDir string `json:"cursor_working_dir"` // 工作目录路径
}

// LoadConfig 加载配置
func LoadConfig() (*Config, error) {
	// 尝试加载.env文件
	if err := godotenv.Load(); err != nil {
		logrus.Debug("No .env file found, using environment variables")
	}

	config := &Config{
		// 设置默认值
		Port:               getEnvAsInt("PORT", 8002),
		Debug:              getEnvAsBool("DEBUG", false),
		APIKey:             getEnv("API_KEY", "0000"),
		Models:             getEnv("MODELS", "claude-3.5-sonnet,gpt-4o,claude-3.5-haiku"),
		SystemPromptInject: getEnv("SYSTEM_PROMPT_INJECT", ""),
		Timeout:            getEnvAsInt("TIMEOUT", 120),
		MaxInputLength:     getEnvAsInt("MAX_INPUT_LENGTH", 200000),

		// Cursor IDE 客户端配置
		CursorAPIURL:     getEnv("CURSOR_API_URL", "https://api2.cursor.sh"),
		CursorToken:      cleanToken(getEnv("CURSOR_TOKEN", "")),
		CursorClientKey:  getEnv("CURSOR_CLIENT_KEY", ""),
		CursorChecksum:   getEnv("CURSOR_CHECKSUM", ""),
		CursorVersion:    getEnv("CURSOR_VERSION", "0.48.6"),
		CursorTimezone:   getEnv("CURSOR_TIMEZONE", "Asia/Shanghai"),
		CursorGhostMode:  getEnvAsBool("CURSOR_GHOST_MODE", true),
		CursorWorkingDir: getEnv("CURSOR_WORKING_DIR", "/c:/Users/Default"),
	}

	// 验证必要的配置
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// validate 验证配置
func (c *Config) validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}

	if c.APIKey == "" {
		return fmt.Errorf("API_KEY is required")
	}

	if c.CursorToken == "" {
		logrus.Warn("CURSOR_TOKEN is not set. You need to provide a valid Cursor JWT token.")
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if c.MaxInputLength <= 0 {
		return fmt.Errorf("max input length must be positive")
	}

	return nil
}

// GetModels 获取模型列表
func (c *Config) GetModels() []string {
	models := strings.Split(c.Models, ",")
	result := make([]string, 0, len(models))
	for _, model := range models {
		if trimmed := strings.TrimSpace(model); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// IsValidModel 检查模型是否有效
func (c *Config) IsValidModel(model string) bool {
	validModels := c.GetModels()
	for _, validModel := range validModels {
		if validModel == model {
			return true
		}
	}
	return false
}

// ToJSON 将配置序列化为JSON（用于调试）
func (c *Config) ToJSON() string {
	// 创建一个副本，隐藏敏感信息
	safeCfg := *c
	safeCfg.APIKey = "***"
	safeCfg.CursorToken = maskToken(c.CursorToken)
	safeCfg.CursorClientKey = maskToken(c.CursorClientKey)
	safeCfg.CursorChecksum = maskToken(c.CursorChecksum)

	data, err := json.MarshalIndent(safeCfg, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling config: %v", err)
	}
	return string(data)
}

// maskToken 隐藏敏感token
func maskToken(token string) string {
	if len(token) <= 16 {
		return "***"
	}
	return token[:8] + "..." + token[len(token)-4:]
}

// cleanToken 清理和标准化 token
// 处理 WorkosCursorSessionToken 格式，包含 %3A%3A (::) 分隔符的情况
func cleanToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}

	// 处理包含 %3A%3A (URL 编码的 ::) 的 token
	// 格式: user_XXXX%3A%3Aactual_token
	if strings.Contains(token, "%3A%3A") {
		parts := strings.Split(token, "%3A%3A")
		if len(parts) > 1 {
			token = parts[len(parts)-1]
		}
	}

	// 处理包含 :: 的 token (未编码版本)
	if strings.Contains(token, "::") {
		parts := strings.Split(token, "::")
		if len(parts) > 1 {
			token = parts[len(parts)-1]
		}
	}

	return strings.TrimSpace(token)
}

// 辅助函数

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt 获取环境变量并转换为int
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		logrus.Warnf("Invalid integer value for %s: %s, using default: %d", key, valueStr, defaultValue)
		return defaultValue
	}

	return value
}

// getEnvAsBool 获取环境变量并转换为bool
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		logrus.Warnf("Invalid boolean value for %s: %s, using default: %t", key, valueStr, defaultValue)
		return defaultValue
	}

	return value
}
