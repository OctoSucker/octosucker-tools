package skill

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool 定义了一个可被 LLM 调用的工具（能力点）
// 一个 Skill 包可以注册多个 Tool，每个 Tool 代表一个独立的可调用能力
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Handler     ToolHandler            `json:"-"`
}

// ToolHandler 是 Tool 的执行函数，ctx 用于取消与超时
type ToolHandler func(ctx context.Context, params map[string]interface{}) (interface{}, error)

// ToolRegistry 管理所有可用的 Tool
// Agent 使用这个注册器加载和调用所有已安装的 Tool
type ToolRegistry struct {
	tools map[string]*Tool
}

// NewToolRegistry 创建新的 Tool 注册表
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*Tool),
	}
}

// Register 注册一个 Tool
func (tr *ToolRegistry) Register(tool *Tool) {
	if tool == nil {
		return
	}
	if tool.Handler == nil {
		panic(fmt.Sprintf("tool %s must have a handler", tool.Name))
	}
	tr.tools[tool.Name] = tool
}

// GetTool 获取指定名称的 Tool
func (tr *ToolRegistry) GetTool(name string) (*Tool, error) {
	tool, exists := tr.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}
	return tool, nil
}

// GetAllTools 获取所有 Tool（用于 Function Calling）
func (tr *ToolRegistry) GetAllTools() []map[string]interface{} {
	var tools []map[string]interface{}
	for _, tool := range tr.tools {
		tools = append(tools, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			},
		})
	}
	return tools
}

// ExecuteTool 执行指定的 Tool，ctx 用于取消与超时
func (tr *ToolRegistry) ExecuteTool(ctx context.Context, name string, argumentsJSON string) (interface{}, error) {
	tool, err := tr.GetTool(name)
	if err != nil {
		return nil, err
	}

	// 解析参数
	var params map[string]interface{}
	if argumentsJSON != "" {
		if err := json.Unmarshal([]byte(argumentsJSON), &params); err != nil {
			return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
		}
	} else {
		params = make(map[string]interface{})
	}

	// 执行 Tool
	return tool.Handler(ctx, params)
}

// GetToolNames 获取所有 Tool 名称（用于 Prompt 描述）
func (tr *ToolRegistry) GetToolNames() []string {
	names := make([]string, 0, len(tr.tools))
	for name := range tr.tools {
		names = append(names, name)
	}
	return names
}
