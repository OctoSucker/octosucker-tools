package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Handler     ToolHandler            `json:"-"`
}

type ToolHandler func(ctx context.Context, params map[string]interface{}) (interface{}, error)

type ToolRegistry struct {
	tools map[string]*Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*Tool),
	}
}

func (tr *ToolRegistry) Register(tool *Tool) {
	if tool == nil {
		return
	}
	if tool.Handler == nil {
		panic(fmt.Sprintf("tool %s must have a handler", tool.Name))
	}
	tr.tools[tool.Name] = tool
}

func (tr *ToolRegistry) GetTool(name string) (*Tool, error) {
	tool, exists := tr.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}
	return tool, nil
}

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

func (tr *ToolRegistry) ExecuteTool(ctx context.Context, name string, argumentsJSON string) (interface{}, error) {
	tool, err := tr.GetTool(name)
	if err != nil {
		return nil, err
	}

	var params map[string]interface{}
	if argumentsJSON != "" {
		if err := json.Unmarshal([]byte(argumentsJSON), &params); err != nil {
			return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
		}
	} else {
		params = make(map[string]interface{})
	}

	return tool.Handler(ctx, params)
}

func (tr *ToolRegistry) GetToolNames() []string {
	names := make([]string, 0, len(tr.tools))
	for name := range tr.tools {
		names = append(names, name)
	}
	return names
}
