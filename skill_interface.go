package skill

import (
	"fmt"
	"sync"
)

// SkillMetadata Skill 元数据
type SkillMetadata struct {
	Name         string                 `json:"name"`         // Skill 名称（包路径）
	Version      string                 `json:"version"`     // Skill 版本
	Description  string                 `json:"description"` // Skill 描述
	Author       string                 `json:"author"`      // Skill 作者
	Tags         []string               `json:"tags"`        // Skill 标签
	Dependencies []string               `json:"dependencies"` // 依赖的其他 Skill
	Config       map[string]interface{} `json:"config"`     // Skill 配置
}

// SkillRegisterFunc 是 skill 包的注册函数类型
// skill 包应该导出一个名为 RegisterSkill 的函数，类型为 SkillRegisterFunc
// 这样可以通过 import _ "skill-package" 的方式自动注册
// agent 参数是 interface{} 类型，Skill 可以通过类型断言来使用 Agent 的功能
// 例如：executor, ok := agent.(AgentToolExecutor)
type SkillRegisterFunc func(registry *ToolRegistry, agent interface{}) error

// SkillLifecycle Skill 生命周期接口
type SkillLifecycle interface {
	// Init 初始化 Skill（在注册时调用）
	Init(config map[string]interface{}) error

	// Cleanup 清理 Skill（在 Agent 关闭时调用）
	Cleanup() error
}

// SkillInfo Skill 信息
type SkillInfo struct {
	Metadata  SkillMetadata
	Register  SkillRegisterFunc
	Lifecycle SkillLifecycle
	Loaded    bool
}

// globalSkillRegistry 全局 skill 注册表
// skill 包在 init() 中调用 RegisterSkill 来注册自己
var (
	globalSkillRegistry = make(map[string]*SkillInfo)
	skillRegistryMutex  sync.RWMutex
)

// RegisterSkill 注册一个 skill（由 skill 包在 init() 中调用）
// name: skill 的名称（建议使用包路径，如 "github.com/OctoSucker/skill-agent-chat"）
// registerFunc: 注册函数，用于注册 skill 提供的工具
func RegisterSkill(name string, registerFunc SkillRegisterFunc) {
	RegisterSkillWithMetadata(name, SkillMetadata{
		Name: name,
	}, registerFunc, nil)
}

// RegisterSkillWithMetadata 注册一个 skill（带元数据）
func RegisterSkillWithMetadata(
	name string,
	metadata SkillMetadata,
	registerFunc SkillRegisterFunc,
	lifecycle SkillLifecycle,
) {
	if name == "" {
		panic("skill name cannot be empty")
	}
	if registerFunc == nil {
		panic(fmt.Sprintf("skill %s: registerFunc cannot be nil", name))
	}

	skillRegistryMutex.Lock()
	defer skillRegistryMutex.Unlock()

	// 检查是否已注册
	if _, exists := globalSkillRegistry[name]; exists {
		panic(fmt.Sprintf("skill %s is already registered", name))
	}

	metadata.Name = name
	globalSkillRegistry[name] = &SkillInfo{
		Metadata:  metadata,
		Register:  registerFunc,
		Lifecycle: lifecycle,
		Loaded:    false,
	}
}

// SkillContext Skill 上下文，传递给注册函数
type SkillContext struct {
	Registry  *ToolRegistry
	Agent     interface{} // Agent 实例（通过类型断言使用）
	Config    map[string]interface{}
	Lifecycle SkillLifecycle
}

// LoadAllRegisteredSkills 加载所有已注册的 skill 到工具注册表
// 这个方法在 Agent 初始化时调用，会自动加载所有通过 import 注册的 skill
// configs: Skill 配置映射，key 为 Skill 名称，value 为配置
// agent: Agent 实例，Skill 可以通过类型断言来使用
func LoadAllRegisteredSkills(toolRegistry *ToolRegistry, agent interface{}, configs map[string]map[string]interface{}) error {
	skillRegistryMutex.RLock()
	skills := make([]*SkillInfo, 0, len(globalSkillRegistry))
	for _, info := range globalSkillRegistry {
		skills = append(skills, info)
	}
	skillRegistryMutex.RUnlock()

	// 加载每个 skill
	for _, skillInfo := range skills {
		if skillInfo.Loaded {
			continue
		}

		// 获取配置
		config := skillInfo.Metadata.Config
		if configs != nil {
			if skillConfig, ok := configs[skillInfo.Metadata.Name]; ok {
				config = skillConfig
			}
		}

		// 初始化 Skill（如果有生命周期接口）
		if skillInfo.Lifecycle != nil {
			if err := skillInfo.Lifecycle.Init(config); err != nil {
				return fmt.Errorf("failed to init skill %s: %w", skillInfo.Metadata.Name, err)
			}
		}

		// 创建 Skill 上下文
		ctx := &SkillContext{
			Registry:  toolRegistry,
			Agent:     agent,
			Config:    config,
			Lifecycle: skillInfo.Lifecycle,
		}
		_ = ctx // 保留用于未来扩展

		// 注册工具（传递 agent 实例）
		if err := skillInfo.Register(toolRegistry, agent); err != nil {
			return fmt.Errorf("failed to register skill %s: %w", skillInfo.Metadata.Name, err)
		}

		skillInfo.Loaded = true
	}

	return nil
}

// GetRegisteredSkillNames 获取所有已注册的 skill 名称
func GetRegisteredSkillNames() []string {
	skillRegistryMutex.RLock()
	defer skillRegistryMutex.RUnlock()

	names := make([]string, 0, len(globalSkillRegistry))
	for name := range globalSkillRegistry {
		names = append(names, name)
	}
	return names
}

// GetSkillInfo 获取 Skill 信息
func GetSkillInfo(name string) (*SkillInfo, error) {
	skillRegistryMutex.RLock()
	defer skillRegistryMutex.RUnlock()

	info, exists := globalSkillRegistry[name]
	if !exists {
		return nil, fmt.Errorf("skill %s not found", name)
	}
	return info, nil
}

// GetAllSkillInfos 获取所有 Skill 信息
func GetAllSkillInfos() map[string]*SkillInfo {
	skillRegistryMutex.RLock()
	defer skillRegistryMutex.RUnlock()

	result := make(map[string]*SkillInfo)
	for name, info := range globalSkillRegistry {
		result[name] = info
	}
	return result
}

// CleanupAllSkills 清理所有 Skill
func CleanupAllSkills() error {
	skillRegistryMutex.RLock()
	skills := make([]*SkillInfo, 0, len(globalSkillRegistry))
	for _, info := range globalSkillRegistry {
		if info.Loaded && info.Lifecycle != nil {
			skills = append(skills, info)
		}
	}
	skillRegistryMutex.RUnlock()

	for _, skillInfo := range skills {
		if err := skillInfo.Lifecycle.Cleanup(); err != nil {
			return fmt.Errorf("failed to cleanup skill %s: %w", skillInfo.Metadata.Name, err)
		}
		skillInfo.Loaded = false
	}

	return nil
}
