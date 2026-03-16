package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

var (
	builtinConfigPath string
	builtinSubmitTask func(string) error
	builtinConfigMu   sync.RWMutex
)

type BuiltinToolProvider struct{}

func (s *BuiltinToolProvider) Init(config map[string]interface{}, submitTask func(string) error) error {
	builtinConfigMu.Lock()
	defer builtinConfigMu.Unlock()
	if config != nil {
		if p, ok := config["config_path"].(string); ok {
			builtinConfigPath = p
		}
	}
	builtinSubmitTask = submitTask
	return nil
}

func (s *BuiltinToolProvider) Cleanup() error {
	return nil
}

func (s *BuiltinToolProvider) Register(registry *ToolRegistry, agent interface{}, providerName string) error {
	registry.RegisterTool(providerName, &Tool{
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

	registry.RegisterTool(providerName, &Tool{
		Name:        "list_tool_providers",
		Description: "列出所有已注册的 Tool provider 及其状态（加载状态、错误信息等）。用于检查哪些成功加载，哪些失败，以及失败原因。",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			all := GetAllToolProviderInfos()
			providerToTools := registry.GetProviderToolShortNames()
			list := make([]map[string]interface{}, 0, len(all))
			for name, info := range all {
				tools := providerToTools[name]
				if tools == nil {
					tools = []string{}
				}
				item := map[string]interface{}{
					"name":        name,
					"description": info.Description,
					"loaded":      info.Loaded,
					"tools":       tools,
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

	registry.RegisterTool(providerName, &Tool{
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
			builtinConfigMu.RLock()
			fn := builtinSubmitTask
			builtinConfigMu.RUnlock()
			err := reloadToolProvider(providerName, registry, agent, config, fn)
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

	registry.RegisterTool(providerName, &Tool{
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
			builtinConfigMu.RLock()
			configPath := builtinConfigPath
			builtinConfigMu.RUnlock()
			if path, ok := params["config_path"].(string); ok && path != "" {
				configPath = path
			}
			if configPath == "" {
				return nil, fmt.Errorf("config_path is required (set in builtin provider config or pass as parameter)")
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

func reloadToolProvider(name string, toolRegistry *ToolRegistry, agent interface{}, config map[string]interface{}, submitTask func(string) error) error {
	toolProviderRegistryMutex.Lock()
	info, exists := globalToolProviderRegistry[name]
	if !exists {
		toolProviderRegistryMutex.Unlock()
		return fmt.Errorf("tool provider %s not found", name)
	}
	toolProviderRegistryMutex.Unlock()

	if info.Loaded && info.Provider != nil {
		if err := info.Provider.Cleanup(); err != nil {
			return fmt.Errorf("failed to cleanup tool provider %s before reload: %w", name, err)
		}
		info.Loaded = false
		info.InitError = nil
	}

	if info.Provider != nil {
		if err := info.Provider.Init(config, submitTask); err != nil {
			info.InitError = err
			return fmt.Errorf("failed to init tool provider %s: %w", name, err)
		}
	}

	if err := info.Provider.Register(toolRegistry, agent, name); err != nil {
		info.InitError = err
		return fmt.Errorf("failed to register tool provider %s: %w", name, err)
	}

	info.Loaded = true
	info.InitError = nil

	return nil
}

func sanitizeConfig(config map[string]interface{}) map[string]interface{} {
	if config == nil {
		return nil
	}
	return sanitizeMap(config)
}

func sanitizeMap(src map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})
	for k, v := range src {
		if isSensitiveKey(k) {
			if s, ok := v.(string); ok {
				sanitized[k] = maskSecret(s)
			} else {
				sanitized[k] = "***"
			}
			continue
		}
		switch vv := v.(type) {
		case map[string]interface{}:
			sanitized[k] = sanitizeMap(vv)
		case []interface{}:
			sanitized[k] = sanitizeSlice(vv)
		default:
			sanitized[k] = v
		}
	}
	return sanitized
}

func sanitizeSlice(src []interface{}) []interface{} {
	out := make([]interface{}, 0, len(src))
	for _, v := range src {
		switch vv := v.(type) {
		case map[string]interface{}:
			out = append(out, sanitizeMap(vv))
		case []interface{}:
			out = append(out, sanitizeSlice(vv))
		default:
			out = append(out, v)
		}
	}
	return out
}

func isSensitiveKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return false
	}
	sensitive := []string{
		"apikey", "api_key", "api-key",
		"token", "bot_token", "access_token", "refresh_token",
		"secret", "client_secret", "password", "passwd", "authorization",
	}
	for _, s := range sensitive {
		if k == s || strings.Contains(k, s) {
			return true
		}
	}
	return false
}

func maskSecret(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 8 {
		return "***"
	}
	return v[:8] + "..."
}

func init() {
	RegisterToolProvider(&ToolProviderInfo{
		Name:        "github.com/OctoSucker/octosucker-tools/builtin",
		Description: "Builtin - 提供管理工具（log_message, list_tool_providers, reload_tool_provider）",
		Provider:    &BuiltinToolProvider{},
	})
}
