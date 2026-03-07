package skill

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// BuiltinSkill 内置 Skill，提供 Skill 系统的管理工具
type BuiltinSkill struct{}

// Init 初始化内置 Skill（不需要配置，总是成功）
func (s *BuiltinSkill) Init(config map[string]interface{}) error {
	return nil
}

// Cleanup 清理内置 Skill
func (s *BuiltinSkill) Cleanup() error {
	return nil
}

// RegisterBuiltinSkill 注册内置 Skill 提供的 Tool
func RegisterBuiltinSkill(registry *ToolRegistry, agent interface{}) error {
	// log_message Tool：让 LLM 可以打印日志，告知用户需要什么配置
	registry.Register(&Tool{
		Name:        "log_message",
		Description: "打印日志消息，用于告知用户需要什么配置或信息。当 Skill 缺少配置时，可以使用此工具告知用户。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"level": map[string]interface{}{
					"type":        "string",
					"description": "日志级别：info, warning, error",
					"enum":        []string{"info", "warning", "error"},
					"default":     "info",
				},
				"message": map[string]interface{}{
					"type":        "string",
					"description": "要打印的日志消息",
				},
			},
			"required": []string{"message"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			message, ok := params["message"].(string)
			if !ok || message == "" {
				return nil, fmt.Errorf("message is required")
			}

			level, _ := params["level"].(string)
			if level == "" {
				level = "info"
			}

			// 根据级别打印日志
			switch level {
			case "error":
				log.Printf("[ERROR] %s", message)
			case "warning":
				log.Printf("[WARNING] %s", message)
			default:
				log.Printf("[INFO] %s", message)
			}

			return map[string]interface{}{
				"success": true,
				"level":   level,
				"message": message,
			}, nil
		},
	})

	// list_skills Tool：列出所有 Skill 及其状态
	registry.Register(&Tool{
		Name:        "list_skills",
		Description: "列出所有已注册的 Skill 及其状态（加载状态、错误信息等）。用于检查哪些 Skill 成功加载，哪些失败，以及失败原因。",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			allSkills := GetAllSkillInfos()
			skillsList := make([]map[string]interface{}, 0, len(allSkills))

			for name, info := range allSkills {
				skillData := map[string]interface{}{
					"name":        name,
					"version":     info.Metadata.Version,
					"description": info.Metadata.Description,
					"author":      info.Metadata.Author,
					"tags":        info.Metadata.Tags,
					"loaded":      info.Loaded,
				}

				if info.InitError != nil {
					skillData["init_error"] = info.InitError.Error()
					skillData["status"] = "failed"
				} else if info.Loaded {
					skillData["status"] = "loaded"
				} else {
					skillData["status"] = "not_loaded"
				}

				skillsList = append(skillsList, skillData)
			}

			return map[string]interface{}{
				"success": true,
				"skills":  skillsList,
				"count":   len(skillsList),
			}, nil
		},
	})

	// reload_skill Tool：重新加载指定的 Skill（使用新配置）
	registry.Register(&Tool{
		Name:        "reload_skill",
		Description: "重新加载指定的 Skill，使用提供的配置。如果 Skill 已经加载，会先清理再重新加载。用于在获取到必要配置后重新启动失败的 Skill。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"skill_name": map[string]interface{}{
					"type":        "string",
					"description": "要重新加载的 Skill 名称（包路径，如 github.com/OctoSucker/skill-telegram）",
				},
				"config": map[string]interface{}{
					"type":        "object",
					"description": "Skill 的新配置（键值对）。如果不提供，则使用 Skill 的默认配置。",
				},
			},
			"required": []string{"skill_name"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			skillName, ok := params["skill_name"].(string)
			if !ok || skillName == "" {
				return nil, fmt.Errorf("skill_name is required")
			}

			var config map[string]interface{}
			if configRaw, ok := params["config"]; ok && configRaw != nil {
				if configMap, ok := configRaw.(map[string]interface{}); ok {
					config = configMap
				}
			}

			err := ReloadSkill(skillName, registry, agent, config)
			if err != nil {
				return map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				}, nil
			}

			return map[string]interface{}{
				"success":    true,
				"skill_name": skillName,
				"message":    fmt.Sprintf("Skill %s reloaded successfully", skillName),
			}, nil
		},
	})

	// read_config_file Tool：读取 Agent 配置文件内容
	registry.Register(&Tool{
		Name:        "read_config_file",
		Description: "读取 Agent 配置文件内容。用于查看当前配置，特别是 Skill 配置（如 Telegram bot_token 等）。敏感信息（如 API keys）会被部分脱敏。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"config_path": map[string]interface{}{
					"type":        "string",
					"description": "配置文件路径（可选，默认使用 Agent 启动时的配置路径）",
				},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			// 尝试从 agent 参数获取 configPath
			var configPath string
			if configPathProvider, ok := agent.(interface {
				GetConfigPath() string
			}); ok {
				configPath = configPathProvider.GetConfigPath()
			}

			// 如果参数中提供了路径，优先使用
			if path, ok := params["config_path"].(string); ok && path != "" {
				configPath = path
			}

			if configPath == "" {
				return nil, fmt.Errorf("config_path is required (either from agent or parameter)")
			}

			// 读取配置文件
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}

			// 解析 JSON
			var config map[string]interface{}
			if err := json.Unmarshal(data, &config); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}

			// 脱敏敏感信息
			sanitizedConfig := sanitizeConfig(config)

			return map[string]interface{}{
				"success":     true,
				"config_path": configPath,
				"config":      sanitizedConfig,
			}, nil
		},
	})

	return nil
}

// sanitizeConfig 脱敏配置文件中的敏感信息
func sanitizeConfig(config map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})
	for k, v := range config {
		switch k {
		case "llm":
			if llmConfig, ok := v.(map[string]interface{}); ok {
				sanitizedLLM := make(map[string]interface{})
				for lk, lv := range llmConfig {
					if lk == "apiKey" {
						// 脱敏 API Key（只显示前 10 个字符）
						if apiKey, ok := lv.(string); ok && len(apiKey) > 10 {
							sanitizedLLM[lk] = apiKey[:10] + "..."
						} else {
							sanitizedLLM[lk] = lv
						}
					} else {
						sanitizedLLM[lk] = lv
					}
				}
				sanitized[k] = sanitizedLLM
			} else {
				sanitized[k] = v
			}
		case "telegram":
			if telegramConfig, ok := v.(map[string]interface{}); ok {
				sanitizedTelegram := make(map[string]interface{})
				for tk, tv := range telegramConfig {
					if tk == "bot_token" {
						// 脱敏 Bot Token（只显示前 10 个字符）
						if botToken, ok := tv.(string); ok && len(botToken) > 10 {
							sanitizedTelegram[tk] = botToken[:10] + "..."
						} else {
							sanitizedTelegram[tk] = tv
						}
					} else {
						sanitizedTelegram[tk] = tv
					}
				}
				sanitized[k] = sanitizedTelegram
			} else {
				sanitized[k] = v
			}
		default:
			sanitized[k] = v
		}
	}
	return sanitized
}

// init 注册内置 Skill
func init() {
	RegisterSkillWithMetadata(
		"github.com/OctoSucker/octosucker-skill/builtin",
		SkillMetadata{
			Name:        "github.com/OctoSucker/octosucker-skill/builtin",
			Version:     "0.1.0",
			Description: "Builtin Skill - 提供 Skill 系统的管理工具（log_message, list_skills, reload_skill）",
			Author:      "OctoSucker",
			Tags:        []string{"builtin", "core", "management"},
		},
		RegisterBuiltinSkill,
		&BuiltinSkill{},
	)
}
