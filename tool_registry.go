package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
	if strings.Contains(name, "/") {
		tool, exists := tr.tools[name]
		if !exists {
			return nil, fmt.Errorf("tool %s not found", name)
		}
		return tool, nil
	}
	var found *Tool
	var fullName string
	for fn, t := range tr.tools {
		if t.Name != name {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("ambiguous tool name %q: use full name (e.g. %s or %s)", name, fullName, fn)
		}
		found = t
		fullName = fn
	}
	if found == nil {
		return nil, fmt.Errorf("tool %s not found", name)
	}
	return found, nil
}

func (tr *ToolRegistry) publicName(fullName string, tool *Tool, shortCount map[string]int) string {
	if shortCount[tool.Name] == 1 {
		return tool.Name
	}
	return fullName
}

func (tr *ToolRegistry) GetAllTools() []map[string]interface{} {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	shortCount := make(map[string]int)
	for _, t := range tr.tools {
		shortCount[t.Name]++
	}
	type item struct {
		publicName string
		tool       *Tool
	}
	var list []item
	for fullName, tool := range tr.tools {
		list = append(list, item{tr.publicName(fullName, tool, shortCount), tool})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].publicName < list[j].publicName })
	var tools []map[string]interface{}
	for _, it := range list {
		tools = append(tools, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        it.publicName,
				"description": it.tool.Description,
				"parameters":  it.tool.Parameters,
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
	shortCount := make(map[string]int)
	for _, t := range tr.tools {
		shortCount[t.Name]++
	}
	names := make([]string, 0, len(tr.tools))
	for fullName, tool := range tr.tools {
		names = append(names, tr.publicName(fullName, tool, shortCount))
	}
	sort.Strings(names)
	return names
}

func (tr *ToolRegistry) GetProviderToolShortNames() map[string][]string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	out := make(map[string][]string)
	for fullName, tool := range tr.tools {
		i := strings.LastIndex(fullName, "/")
		if i < 0 {
			continue
		}
		provider := fullName[:i]
		out[provider] = append(out[provider], tool.Name)
	}
	for p := range out {
		sort.Strings(out[p])
	}
	return out
}
