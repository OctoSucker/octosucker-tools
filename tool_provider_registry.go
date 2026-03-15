package tools

import (
	"fmt"
	"sync"
)

var (
	globalToolProviderRegistry = make(map[string]*ToolProviderInfo)
	toolProviderRegistryMutex  sync.RWMutex
)

type ToolProviderInfo struct {
	Name        string
	Description string
	Provider    ToolProvider
	Loaded      bool
	InitError   error
}

type ToolProvider interface {
	Init(config map[string]interface{}, submitTask func(string) error) error
	Cleanup() error
	Register(registry *ToolRegistry, agent interface{}, providerName string) error
}

func RegisterToolProvider(info *ToolProviderInfo) {
	if info == nil || info.Name == "" {
		panic("tool provider name cannot be empty")
	}
	if info.Provider == nil {
		panic(fmt.Sprintf("tool provider %s: provider cannot be nil", info.Name))
	}
	toolProviderRegistryMutex.Lock()
	defer toolProviderRegistryMutex.Unlock()
	if _, exists := globalToolProviderRegistry[info.Name]; exists {
		panic(fmt.Sprintf("tool provider %s is already registered", info.Name))
	}
	globalToolProviderRegistry[info.Name] = info
}

func LoadAllToolProviders(toolRegistry *ToolRegistry, agent interface{}, configs map[string]map[string]interface{}, submitTask func(string) error) map[string]error {
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

		var config map[string]interface{}
		if configs != nil {
			config, _ = configs[info.Name]
		}

		if info.Provider != nil {
			if err := info.Provider.Init(config, submitTask); err != nil {
				info.InitError = err
				failed[info.Name] = err
				continue
			}
		}

		if err := info.Provider.Register(toolRegistry, agent, info.Name); err != nil {
			info.InitError = err
			failed[info.Name] = err
			continue
		}

		info.Loaded = true
		info.InitError = nil
	}

	return failed
}

func GetAllToolProviderInfos() map[string]*ToolProviderInfo {
	toolProviderRegistryMutex.RLock()
	defer toolProviderRegistryMutex.RUnlock()
	result := make(map[string]*ToolProviderInfo, len(globalToolProviderRegistry))
	for name, info := range globalToolProviderRegistry {
		result[name] = info
	}
	return result
}
