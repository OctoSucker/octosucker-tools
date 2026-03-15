# octosucker-tools

OctoSucker 的 **Tool Provider 内核库**。  
提供统一的 `ToolRegistry`、Tool Provider 注册 / 生命周期管理等能力，供各个 `tools-*` 仓库和 Agent 复用。

## 核心概念

- **Tool**（工具）：
  - 对 LLM 暴露的能力点（一个函数调用）。
  - 包含：`Name`、`Description`、`Parameters`（JSON Schema 风格）、`Handler(ctx, params)`。
- **ToolRegistry**：
  - 维护一组 Tool，支持注册和按名称执行。
  - Agent 在运行时使用它把工具列表传入 LLM（Function Calling），并在有 tool_calls 时分发执行。
- **Tool Provider**：
  - 一组相关工具 + 生命周期逻辑（Init/Cleanup）。
  - 例如：`tools-fs`、`tools-web`、`tools-exec`、`tools-remember`、`tools-cron`、`tools-mcp`、`tools-telegram`。

## 用法概览

### 1. 在 Tool Provider 仓库中注册

```go
package mytools

import (
    "context"

    tools "github.com/OctoSucker/octosucker-tools"
)

type MyProvider struct{}

func (p *MyProvider) Init(config map[string]interface{}, submitTask func(string) error) error { return nil }
func (p *MyProvider) Cleanup() error                                                       { return nil }
func (p *MyProvider) Register(registry *tools.ToolRegistry, agent interface{}, providerName string) error {
    registry.RegisterTool(providerName, &tools.Tool{
        Name:        "my_tool",
        Description: "示例工具",
        Parameters: map[string]interface{}{
            "type":       "object",
            "properties": map[string]interface{}{},
        },
        Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
            return map[string]interface{}{"ok": true}, nil
        },
    })
    return nil
}

func init() {
    tools.RegisterToolProvider(&tools.ToolProviderInfo{
        Name:        "github.com/your-org/tools-my",
        Description: "My custom tools",
        Provider:    &MyProvider{},
    })
}
```

### 2. 在 Agent 中加载所有 Tool Provider

在 Agent 侧（例如 OctoSucker 主工程）：

```go
import (
    tools "github.com/OctoSucker/octosucker-tools"
    _ "github.com/OctoSucker/tools-fs"
    _ "github.com/OctoSucker/tools-web"
    // ...
)

registry := tools.NewToolRegistry()
failed := tools.LoadAllToolProviders(registry, agentInstance, toolProviderConfigs)
```

其中 `toolProviderConfigs` 来自配置文件（如 `config/agent_config.json` 中的 `"tool_providers"` 字段）。

## 设计原则

- **平台无关**：不依赖具体 Agent 实现，仅依赖一个 `interface{}` 作为 Agent 句柄，必要能力通过类型断言接口暴露。
- **自动注册**：Tool Provider 通过 `init()` 调用 `RegisterToolProvider*` 自动注册。
- **生命周期管理**：统一管理 Init/Cleanup，可在运行时通过内建工具触发 reload。
- **简洁 JSON Schema**：`Parameters` 与 OpenAI Function Calling 兼容，方便直接传入 LLM。

## License

MIT
