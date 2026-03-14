package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type BuiltinToolProvider struct{}

func (s *BuiltinToolProvider) Init(config map[string]interface{}) error {
	return nil
}

func (s *BuiltinToolProvider) Cleanup() error {
	return nil
}

func RegisterBuiltinToolProvider(registry *ToolRegistry, agent interface{}) error {
	registry.Register(&Tool{
		Name:        "log_message",
		Description: "打印日志消息，用于告知用户需要什么配置或信息。当 Tool provider 缺少配置时，可以使用此工具告知用户。",
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
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			message, ok := params["message"].(string)
			if !ok || message == "" {
				return nil, fmt.Errorf("message is required")
			}

			level, _ := params["level"].(string)
			if level == "" {
				level = "info"
			}

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

	registry.Register(&Tool{
		Name:        "list_tool_providers",
		Description: "列出所有已注册的 Tool provider 及其状态（加载状态、错误信息等）。用于检查哪些成功加载，哪些失败，以及失败原因。",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			all := GetAllToolProviderInfos()
			list := make([]map[string]interface{}, 0, len(all))

			for name, info := range all {
				item := map[string]interface{}{
					"name":        name,
					"version":     info.Metadata.Version,
					"description": info.Metadata.Description,
					"author":      info.Metadata.Author,
					"tags":        info.Metadata.Tags,
					"loaded":      info.Loaded,
				}

				if info.InitError != nil {
					item["init_error"] = info.InitError.Error()
					item["status"] = "failed"
				} else if info.Loaded {
					item["status"] = "loaded"
				} else {
					item["status"] = "not_loaded"
				}

				list = append(list, item)
			}

			return map[string]interface{}{
				"success":        true,
				"tool_providers": list,
				"count":          len(list),
			}, nil
		},
	})

	registry.Register(&Tool{
		Name:        "reload_tool_provider",
		Description: "重新加载指定的 Tool provider，使用提供的配置。若已加载会先清理再加载。用于在获取到必要配置后重新启动失败的 provider。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"provider_name": map[string]interface{}{
					"type":        "string",
					"description": "要重新加载的 provider 名称（模块路径，如 github.com/OctoSucker/tools-telegram）",
				},
				"config": map[string]interface{}{
					"type":        "object",
					"description": "新配置（键值对）。不提供则使用默认配置。",
				},
			},
			"required": []string{"provider_name"},
		},
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			providerName, ok := params["provider_name"].(string)
			if !ok || providerName == "" {
				return nil, fmt.Errorf("provider_name is required")
			}

			var config map[string]interface{}
			if configRaw, ok := params["config"]; ok && configRaw != nil {
				if configMap, ok := configRaw.(map[string]interface{}); ok {
					config = configMap
				}
			}

			err := ReloadToolProvider(providerName, registry, agent, config)
			if err != nil {
				return map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				}, nil
			}

			return map[string]interface{}{
				"success":       true,
				"provider_name": providerName,
				"message":       fmt.Sprintf("Tool provider %s reloaded successfully", providerName),
			}, nil
		},
	})

	registry.Register(&Tool{
		Name:        "read_config_file",
		Description: "读取 Agent 配置文件内容。用于查看当前配置，特别是 Tool provider 配置（如 Telegram bot_token 等）。敏感信息（如 API keys）会被部分脱敏。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"config_path": map[string]interface{}{
					"type":        "string",
					"description": "配置文件路径（可选，默认使用 Agent 启动时的配置路径）",
				},
			},
		},
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			var configPath string
			if configPathProvider, ok := agent.(ConfigPathProvider); ok {
				configPath = configPathProvider.GetConfigPath()
			}

			if path, ok := params["config_path"].(string); ok && path != "" {
				configPath = path
			}

			if configPath == "" {
				return nil, fmt.Errorf("config_path is required (either from agent or parameter)")
			}

			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}

			var config map[string]interface{}
			if err := json.Unmarshal(data, &config); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}

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

func sanitizeConfig(config map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})
	for k, v := range config {
		switch k {
		case "llm":
			if llmConfig, ok := v.(map[string]interface{}); ok {
				sanitizedLLM := make(map[string]interface{})
				for lk, lv := range llmConfig {
					if lk == "apiKey" {
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

func init() {
	RegisterToolProviderWithMetadata(
		"github.com/OctoSucker/octosucker-tools/builtin",
		ToolProviderMetadata{
			Name:        "github.com/OctoSucker/octosucker-tools/builtin",
			Version:     "0.1.0",
			Description: "Builtin - 提供管理工具（log_message, list_tool_providers, reload_tool_provider）",
			Author:      "OctoSucker",
			Tags:        []string{"builtin", "core", "management"},
		},
		RegisterBuiltinToolProvider,
		&BuiltinToolProvider{},
	)
}
