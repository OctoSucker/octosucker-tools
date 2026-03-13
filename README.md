# octosucker-skill

OctoSucker Skill 接口包 - 定义 Skill 开发的标准接口和 Skill 注册机制

## 概述

`octosucker-skill` 是 OctoSucker AI Agent 平台的 Skill 开发基础库。它定义了：

- **Tool 系统**: `Tool`, `ToolHandler`, `ToolRegistry` - LLM 可调用工具的定义和注册机制
- **Skill 接口**: `SkillLifecycle`, `SkillMetadata`, `SkillRegisterFunc` - Skill 包生命周期和注册接口（一个 Skill 包可以注册多个 Tool）
- **Skill 注册机制**: 自动发现和加载已注册的 Skill 包

## 核心组件

### Tool 系统

- `Tool`: 定义可被 LLM 调用的工具（能力点）
- `ToolRegistry`: 管理所有可用的 Tool，Agent 使用它加载和调用 Tool
- `ToolHandler`: Tool 执行函数类型

### Skill 接口

- `SkillLifecycle`: Skill 生命周期接口（Init, Cleanup）
- `SkillMetadata`: Skill 元数据（名称、版本、描述等）
- `SkillRegisterFunc`: Skill 注册函数类型

### Skill 注册

- `RegisterSkill`: 注册一个 Skill
- `RegisterSkillWithMetadata`: 注册一个 Skill（带元数据）
- `LoadAllRegisteredSkills`: 加载所有已注册的 Skill

## 使用方式

### 在 Skill 包中使用

```go
package main

import (
    "context"

    "github.com/OctoSucker/octosucker-skill"
)

// RegisterMySkill 建议命名为 RegisterXxxSkill，在 init 中注册到 skill 包。
func RegisterMySkill(registry *skill.ToolRegistry, agent interface{}) error {
    // 使用 Agent 能力时通过 skill 包定义的接口做类型断言，例如：
    // if t, ok := agent.(skill.TaskSubmitter); ok { t.SubmitTask(input) }
    registry.Register(&skill.Tool{
        Name:        "my_tool",
        Description: "我的 Tool 能力点",
        Parameters: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{},
            "required": []string{},
        },
        Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
            return nil, nil
        },
    })
    return nil
}

func init() {
    skill.RegisterSkillWithMetadata(
        "github.com/your-org/skill-name",
        skill.SkillMetadata{
            Name:        "github.com/your-org/skill-name",
            Version:     "0.1.0",
            Description: "Skill 描述",
            Author:      "作者",
            Tags:        []string{"tag1", "tag2"},
        },
        RegisterMySkill,
        nil, // 或实现 SkillLifecycle
    )
}
```

### 在 Agent 平台中使用

```go
import (
    "github.com/OctoSucker/octosucker-skill"
    _ "github.com/OctoSucker/skill-agent-chat" // 导入 Skill
)

// 创建 Tool 注册表
registry := skill.NewToolRegistry()

// 加载所有已注册的 Skill 包，并让它们向 ToolRegistry 注册 Tool
configs := map[string]map[string]interface{}{
    "github.com/OctoSucker/skill-agent-chat": {
        "name": "My Agent",
        // ... 其他配置
    },
}

err := skill.LoadAllRegisteredSkills(registry, agentInstance, configs)
```

## 设计原则

1. **独立性**: Skill 接口包不依赖 Agent 平台的具体实现
2. **类型安全**: 通过 `interface{}` 和类型断言实现灵活的 Agent 交互
3. **自动注册**: Skill 通过 `init()` 函数自动注册
4. **生命周期管理**: 支持 Skill 的初始化和清理

## Agent 能力接口

Skill 通过类型断言使用 Agent 提供的能力，接口定义在本包中（见 `TaskSubmitter`、`ConfigPathProvider` 等），避免各 Skill 分散定义。平台实现这些接口，Skill 只依赖本包。

## Tool Parameters 约定

`Parameters` 须与 OpenAI Function Calling 兼容的 JSON Schema：

- 必须包含 `"type": "object"` 和 `"properties"`
- 必填参数放在 `"required"` 数组
- 枚举使用 `"enum"`；可选默认值使用 `"default"`
- 避免使用 `additionalProperties` 等可能导致部分模型报错的字段

## 注意事项

- 注册函数建议命名为 `RegisterXxxSkill`，并在 `init()` 中调用 `RegisterSkill` / `RegisterSkillWithMetadata`（不要导出名为 `RegisterSkill` 的符号）
- Tool 的 `Handler` 签名为 `func(ctx context.Context, params map[string]interface{}) (interface{}, error)`，需使用 `ctx` 处理超时与取消
- Tool 名称全局唯一，建议带 Skill 或语义前缀以防冲突（如 `telegram_send_message`、`read_file`）
- `SkillMetadata.Dependencies` 目前仅作文档/展示用，加载时未按依赖顺序执行

## License

MIT
