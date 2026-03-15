package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Handler     ToolHandler            `json:"-"`
}


type ToolHandler func(ctx context.Context, params map[string]interface{}) (interface{}, error)

type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]*Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*Tool),
	}
}

func (tr *ToolRegistry) RegisterTool(providerName string, tool *Tool) {
	if tool == nil {
		return
	}
	if tool.Handler == nil {
		panic(fmt.Sprintf("tool %s must have a handler", tool.Name))
	}
	fullName := providerName + "/" + tool.Name
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.tools[fullName] = tool
}

func (tr *ToolRegistry) GetTool(name string) (*Tool, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	tool, exists := tr.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}
	return tool, nil
}

func (tr *ToolRegistry) GetAllTools() []map[string]interface{} {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	var tools []map[string]interface{}
	names := make([]string, 0, len(tr.tools))
	for name := range tr.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		tool := tr.tools[name]
		tools = append(tools, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        name,
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
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	names := make([]string, 0, len(tr.tools))
	for name := range tr.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
