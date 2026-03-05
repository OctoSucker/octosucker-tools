package skill

import (
	"encoding/json"
	"fmt"
)

// Tool 定义了一个可被 LLM 调用的工具
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Handler     ToolHandler            `json:"-"`
}

// ToolHandler 是工具的执行函数
type ToolHandler func(params map[string]interface{}) (interface{}, error)

// ToolRegistry 管理所有可用的工具
type ToolRegistry struct {
	tools map[string]*Tool
}

// NewToolRegistry 创建新的工具注册表
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*Tool),
	}
}

// Register 注册一个工具
func (tr *ToolRegistry) Register(tool *Tool) {
	if tool == nil {
		return
	}
	if tool.Handler == nil {
		panic(fmt.Sprintf("tool %s must have a handler", tool.Name))
	}
	tr.tools[tool.Name] = tool
}

// GetTool 获取指定名称的工具
func (tr *ToolRegistry) GetTool(name string) (*Tool, error) {
	tool, exists := tr.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}
	return tool, nil
}

// GetAllTools 获取所有工具（用于 Function Calling）
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

// ExecuteTool 执行指定的工具
func (tr *ToolRegistry) ExecuteTool(name string, argumentsJSON string) (interface{}, error) {
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

	// 执行工具
	return tool.Handler(params)
}

// GetToolNames 获取所有工具名称（用于 Prompt 描述）
func (tr *ToolRegistry) GetToolNames() []string {
	names := make([]string, 0, len(tr.tools))
	for name := range tr.tools {
		names = append(names, name)
	}
	return names
}
