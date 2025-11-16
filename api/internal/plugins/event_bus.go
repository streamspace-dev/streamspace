package plugins

import (
	"log"
	"sync"
)

// EventBus manages event distribution to plugins
type EventBus struct {
	subscribers map[string][]EventHandler
	mu          sync.RWMutex
}

// EventHandler is a function that handles an event
type EventHandler func(data interface{}) error

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]EventHandler),
	}
}

// Subscribe registers a handler for an event type
func (bus *EventBus) Subscribe(eventType string, pluginName string, handler EventHandler) {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	key := eventType + ":" + pluginName
	bus.subscribers[key] = append(bus.subscribers[key], handler)

	log.Printf("[EventBus] Plugin %s subscribed to %s", pluginName, eventType)
}

// Unsubscribe removes a handler
func (bus *EventBus) Unsubscribe(eventType string, pluginName string) {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	key := eventType + ":" + pluginName
	delete(bus.subscribers, key)

	log.Printf("[EventBus] Plugin %s unsubscribed from %s", pluginName, eventType)
}

// UnsubscribeAll removes all handlers for a plugin
func (bus *EventBus) UnsubscribeAll(pluginName string) {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	toDelete := []string{}
	for key := range bus.subscribers {
		// Keys are in format "eventType:pluginName"
		for i := len(key) - 1; i >= 0; i-- {
			if key[i] == ':' {
				if key[i+1:] == pluginName {
					toDelete = append(toDelete, key)
				}
				break
			}
		}
	}

	for _, key := range toDelete {
		delete(bus.subscribers, key)
	}

	log.Printf("[EventBus] Unsubscribed plugin %s from all events", pluginName)
}

// Emit publishes an event to all subscribers
func (bus *EventBus) Emit(eventType string, data interface{}) {
	bus.mu.RLock()
	handlers := make([]EventHandler, 0)

	// Collect all handlers for this event type
	for key, subs := range bus.subscribers {
		// Check if key starts with eventType
		if len(key) >= len(eventType) && key[:len(eventType)] == eventType {
			handlers = append(handlers, subs...)
		}
	}
	bus.mu.RUnlock()

	// Call all handlers concurrently
	var wg sync.WaitGroup
	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[EventBus] Handler panicked on event %s: %v", eventType, r)
				}
			}()

			if err := h(data); err != nil {
				log.Printf("[EventBus] Handler error on event %s: %v", eventType, err)
			}
		}(handler)
	}

	// Don't wait for all handlers to complete (async)
}

// EmitSync publishes an event and waits for all handlers to complete
func (bus *EventBus) EmitSync(eventType string, data interface{}) []error {
	bus.mu.RLock()
	handlers := make([]EventHandler, 0)

	for key, subs := range bus.subscribers {
		if len(key) >= len(eventType) && key[:len(eventType)] == eventType {
			handlers = append(handlers, subs...)
		}
	}
	bus.mu.RUnlock()

	// Call all handlers and collect errors
	errors := make([]error, 0)
	var mu sync.Mutex

	var wg sync.WaitGroup
	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					errors = append(errors, fmt.Errorf("handler panicked: %v", r))
					mu.Unlock()
				}
			}()

			if err := h(data); err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}(handler)
	}

	wg.Wait()
	return errors
}

// PluginEvents provides event API for plugins
type PluginEvents struct {
	bus        *EventBus
	pluginName string
}

// NewPluginEvents creates a new plugin events instance
func NewPluginEvents(bus *EventBus, pluginName string) *PluginEvents {
	return &PluginEvents{
		bus:        bus,
		pluginName: pluginName,
	}
}

// On registers an event handler
func (pe *PluginEvents) On(eventType string, handler func(data interface{}) error) {
	pe.bus.Subscribe(eventType, pe.pluginName, handler)
}

// Off removes an event handler
func (pe *PluginEvents) Off(eventType string) {
	pe.bus.Unsubscribe(eventType, pe.pluginName)
}

// Emit emits an event (plugins can emit custom events)
func (pe *PluginEvents) Emit(eventType string, data interface{}) {
	// Prefix with plugin name to namespace custom events
	pe.bus.Emit("plugin."+pe.pluginName+"."+eventType, data)
}
