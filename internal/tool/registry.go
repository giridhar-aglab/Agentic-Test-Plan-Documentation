package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/llm"
)

// Registry holds every tool available to one agent, already wrapped in its
// policy. Registries are scoped per agent: the Surveyor cannot reach the
// renderer, and the publisher's registry is not constructed at all until a
// human verdict exists. Scoping is a loop guard as much as a safety measure —
// an agent cannot wander into another phase's work.
type Registry struct {
	mutex           sync.RWMutex
	toolsByName     map[string]*guarded
	registeredOrder []string
	nowFunc         func() time.Time
}

// NewRegistry builds an empty registry.
func NewRegistry() *Registry {
	return &Registry{toolsByName: map[string]*guarded{}, nowFunc: time.Now}
}

// Register adds a tool under the given policy. Registering the same name twice
// is a programming error and panics, because a silently shadowed tool is far
// harder to diagnose than a crash at startup.
func (registry *Registry) Register(registeredTool Tool, policy Policy) {
	registry.mutex.Lock()
	defer registry.mutex.Unlock()
	toolName := registeredTool.Name()
	if _, alreadyPresent := registry.toolsByName[toolName]; alreadyPresent {
		panic("tool: duplicate registration for " + toolName)
	}
	registry.toolsByName[toolName] = &guarded{
		inner:  registeredTool,
		policy: policy.withDefaults(),
		now:    registry.nowFunc,
	}
	registry.registeredOrder = append(registry.registeredOrder, toolName)
}

// Scoped returns a new registry containing only the named tools, sharing the
// same underlying breaker state. This is how one agent's toolset is carved out
// of the full set without losing what the policy layer has learned.
func (registry *Registry) Scoped(toolNames ...string) (*Registry, error) {
	registry.mutex.RLock()
	defer registry.mutex.RUnlock()

	scopedRegistry := &Registry{toolsByName: map[string]*guarded{}, nowFunc: registry.nowFunc}
	for _, toolName := range toolNames {
		guardedTool, found := registry.toolsByName[toolName]
		if !found {
			return nil, fmt.Errorf("tool: cannot scope unknown tool %q", toolName)
		}
		scopedRegistry.toolsByName[toolName] = guardedTool
		scopedRegistry.registeredOrder = append(scopedRegistry.registeredOrder, toolName)
	}
	return scopedRegistry, nil
}

// Has reports whether a tool is in scope.
func (registry *Registry) Has(toolName string) bool {
	registry.mutex.RLock()
	defer registry.mutex.RUnlock()
	_, found := registry.toolsByName[toolName]
	return found
}

// Names returns the in-scope tool names in registration order.
func (registry *Registry) Names() []string {
	registry.mutex.RLock()
	defer registry.mutex.RUnlock()
	namesCopy := make([]string, len(registry.registeredOrder))
	copy(namesCopy, registry.registeredOrder)
	return namesCopy
}

// Schemas returns the tools as the model should see them, in deterministic
// order so prompt caching works. A tool whose breaker is open is omitted: the
// model should not be offered a capability that cannot currently work.
func (registry *Registry) Schemas() []llm.ToolSchema {
	registry.mutex.RLock()
	defer registry.mutex.RUnlock()

	schemas := make([]llm.ToolSchema, 0, len(registry.toolsByName))
	for _, toolName := range registry.registeredOrder {
		guardedTool := registry.toolsByName[toolName]
		if guardedTool.breakerOpen() {
			continue
		}
		schemas = append(schemas, llm.ToolSchema{
			Name:        guardedTool.Name(),
			Description: guardedTool.Description(),
			InputSchema: guardedTool.InputSchema(),
		})
	}
	sort.Slice(schemas, func(leftIndex, rightIndex int) bool {
		return schemas[leftIndex].Name < schemas[rightIndex].Name
	})
	return schemas
}

// DegradedTools lists tools currently out of service, for the report's gaps
// section. A plan built without git history should say so.
func (registry *Registry) DegradedTools() []string {
	registry.mutex.RLock()
	defer registry.mutex.RUnlock()
	degradedNames := []string{}
	for _, toolName := range registry.registeredOrder {
		if registry.toolsByName[toolName].breakerOpen() {
			degradedNames = append(degradedNames, toolName)
		}
	}
	return degradedNames
}

// Invoke dispatches a call through the policy layer.
func (registry *Registry) Invoke(ctx context.Context, toolName string, arguments json.RawMessage) (Result, error) {
	registry.mutex.RLock()
	guardedTool, found := registry.toolsByName[toolName]
	registry.mutex.RUnlock()

	if !found {
		// Out of scope is a correctable mistake: tell the model what it may
		// actually call rather than failing the run.
		return Result{}, Correctable(toolName, fmt.Sprintf(
			"no such tool %q; available tools are %v", toolName, registry.Names()))
	}
	return guardedTool.Invoke(ctx, arguments)
}
