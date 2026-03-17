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

func GetAllToolProviderInfos() map[string]*ToolProviderInfo {
	toolProviderRegistryMutex.RLock()
	defer toolProviderRegistryMutex.RUnlock()
	result := make(map[string]*ToolProviderInfo, len(globalToolProviderRegistry))
	for name, info := range globalToolProviderRegistry {
		result[name] = info
	}
	return result
}
