package tools

import (
	"fmt"
	"sync"
)

type ToolProviderMetadata struct {
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description"`
	Author       string                 `json:"author"`
	Tags         []string               `json:"tags"`
	Dependencies []string               `json:"dependencies"`
	Config       map[string]interface{} `json:"config"`
}

type ToolProviderRegisterFunc func(registry *ToolRegistry, agent interface{}) error

type ToolProviderLifecycle interface {
	Init(config map[string]interface{}) error
	Cleanup() error
}

type ToolProviderInfo struct {
	Metadata  ToolProviderMetadata
	Register  ToolProviderRegisterFunc
	Lifecycle ToolProviderLifecycle
	Loaded    bool
	InitError error
}

var (
	globalToolProviderRegistry = make(map[string]*ToolProviderInfo)
	toolProviderRegistryMutex  sync.RWMutex
)

func RegisterToolProvider(name string, registerFunc ToolProviderRegisterFunc) {
	RegisterToolProviderWithMetadata(name, ToolProviderMetadata{
		Name: name,
	}, registerFunc, nil)
}

func RegisterToolProviderWithMetadata(
	name string,
	metadata ToolProviderMetadata,
	registerFunc ToolProviderRegisterFunc,
	lifecycle ToolProviderLifecycle,
) {
	if name == "" {
		panic("tool provider name cannot be empty")
	}
	if registerFunc == nil {
		panic(fmt.Sprintf("tool provider %s: registerFunc cannot be nil", name))
	}

	toolProviderRegistryMutex.Lock()
	defer toolProviderRegistryMutex.Unlock()

	if _, exists := globalToolProviderRegistry[name]; exists {
		panic(fmt.Sprintf("tool provider %s is already registered", name))
	}

	metadata.Name = name
	globalToolProviderRegistry[name] = &ToolProviderInfo{
		Metadata:  metadata,
		Register:  registerFunc,
		Lifecycle: lifecycle,
		Loaded:    false,
		InitError: nil,
	}
}

func LoadAllToolProviders(toolRegistry *ToolRegistry, agent interface{}, configs map[string]map[string]interface{}) map[string]error {
	toolProviderRegistryMutex.RLock()
	providers := make([]*ToolProviderInfo, 0, len(globalToolProviderRegistry))
	for _, info := range globalToolProviderRegistry {
		providers = append(providers, info)
	}
	toolProviderRegistryMutex.RUnlock()

	failed := make(map[string]error)

	for _, info := range providers {
		if info.Loaded {
			continue
		}

		config := info.Metadata.Config
		if configs != nil {
			if providerConfig, ok := configs[info.Metadata.Name]; ok {
				config = providerConfig
			}
		}

		if info.Lifecycle != nil {
			if err := info.Lifecycle.Init(config); err != nil {
				info.InitError = err
				failed[info.Metadata.Name] = err
				continue
			}
		}

		if err := info.Register(toolRegistry, agent); err != nil {
			info.InitError = err
			failed[info.Metadata.Name] = err
			continue
		}

		info.Loaded = true
		info.InitError = nil
	}

	return failed
}

func GetRegisteredToolProviderNames() []string {
	toolProviderRegistryMutex.RLock()
	defer toolProviderRegistryMutex.RUnlock()

	names := make([]string, 0, len(globalToolProviderRegistry))
	for name := range globalToolProviderRegistry {
		names = append(names, name)
	}
	return names
}

func GetToolProviderInfo(name string) (*ToolProviderInfo, error) {
	toolProviderRegistryMutex.RLock()
	defer toolProviderRegistryMutex.RUnlock()

	info, exists := globalToolProviderRegistry[name]
	if !exists {
		return nil, fmt.Errorf("tool provider %s not found", name)
	}
	return info, nil
}

func GetAllToolProviderInfos() map[string]*ToolProviderInfo {
	toolProviderRegistryMutex.RLock()
	defer toolProviderRegistryMutex.RUnlock()

	result := make(map[string]*ToolProviderInfo)
	for name, info := range globalToolProviderRegistry {
		result[name] = info
	}
	return result
}

func CleanupAllToolProviders() error {
	toolProviderRegistryMutex.RLock()
	providers := make([]*ToolProviderInfo, 0, len(globalToolProviderRegistry))
	for _, info := range globalToolProviderRegistry {
		if info.Loaded && info.Lifecycle != nil {
			providers = append(providers, info)
		}
	}
	toolProviderRegistryMutex.RUnlock()

	for _, info := range providers {
		if err := info.Lifecycle.Cleanup(); err != nil {
			return fmt.Errorf("failed to cleanup tool provider %s: %w", info.Metadata.Name, err)
		}
		info.Loaded = false
		info.InitError = nil
	}

	return nil
}

func ReloadToolProvider(name string, toolRegistry *ToolRegistry, agent interface{}, config map[string]interface{}) error {
	toolProviderRegistryMutex.Lock()
	info, exists := globalToolProviderRegistry[name]
	if !exists {
		toolProviderRegistryMutex.Unlock()
		return fmt.Errorf("tool provider %s not found", name)
	}
	toolProviderRegistryMutex.Unlock()

	if info.Loaded && info.Lifecycle != nil {
		if err := info.Lifecycle.Cleanup(); err != nil {
			return fmt.Errorf("failed to cleanup tool provider %s before reload: %w", name, err)
		}
		info.Loaded = false
		info.InitError = nil
	}

	useConfig := config
	if useConfig == nil {
		useConfig = info.Metadata.Config
	}

	if info.Lifecycle != nil {
		if err := info.Lifecycle.Init(useConfig); err != nil {
			info.InitError = err
			return fmt.Errorf("failed to init tool provider %s: %w", name, err)
		}
	}

	if err := info.Register(toolRegistry, agent); err != nil {
		info.InitError = err
		return fmt.Errorf("failed to register tool provider %s: %w", name, err)
	}

	info.Loaded = true
	info.InitError = nil

	return nil
}
