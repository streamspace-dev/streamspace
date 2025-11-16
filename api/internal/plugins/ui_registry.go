package plugins

import (
	"fmt"
	"log"
	"sync"
)

// UIRegistry manages plugin UI component registrations
type UIRegistry struct {
	widgets      map[string]*UIWidget
	pages        map[string]*UIPage
	adminPages   map[string]*UIAdminPage
	menuItems    map[string]*UIMenuItem
	adminWidgets map[string]*UIWidget
	mu           sync.RWMutex
}

// UIWidget represents a dashboard widget
type UIWidget struct {
	PluginName  string
	ID          string
	Title       string
	Component   string
	Position    string // "top", "sidebar", "bottom"
	Width       string // "full", "half", "third"
	Icon        string
	Permissions []string
}

// UIPage represents a user-facing page
type UIPage struct {
	PluginName  string
	ID          string
	Title       string
	Path        string
	Component   string
	Icon        string
	MenuLabel   string
	Permissions []string
}

// UIAdminPage represents an admin page
type UIAdminPage struct {
	PluginName  string
	ID          string
	Title       string
	Path        string
	Component   string
	Icon        string
	MenuLabel   string
	Permissions []string
	Order       int
}

// UIMenuItem represents a menu item
type UIMenuItem struct {
	PluginName  string
	ID          string
	Label       string
	Path        string
	Icon        string
	Component   string
	Order       int
	Permissions []string
}

// NewUIRegistry creates a new UI registry
func NewUIRegistry() *UIRegistry {
	return &UIRegistry{
		widgets:      make(map[string]*UIWidget),
		pages:        make(map[string]*UIPage),
		adminPages:   make(map[string]*UIAdminPage),
		menuItems:    make(map[string]*UIMenuItem),
		adminWidgets: make(map[string]*UIWidget),
	}
}

// RegisterWidget registers a dashboard widget
func (r *UIRegistry) RegisterWidget(pluginName string, widget *UIWidget) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s", pluginName, widget.ID)

	if _, exists := r.widgets[key]; exists {
		return fmt.Errorf("widget %s already registered by plugin %s", widget.ID, pluginName)
	}

	widget.PluginName = pluginName
	r.widgets[key] = widget

	log.Printf("[UI Registry] Registered widget: %s (plugin: %s)", widget.ID, pluginName)
	return nil
}

// RegisterAdminWidget registers an admin dashboard widget
func (r *UIRegistry) RegisterAdminWidget(pluginName string, widget *UIWidget) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s", pluginName, widget.ID)

	if _, exists := r.adminWidgets[key]; exists {
		return fmt.Errorf("admin widget %s already registered by plugin %s", widget.ID, pluginName)
	}

	widget.PluginName = pluginName
	r.adminWidgets[key] = widget

	log.Printf("[UI Registry] Registered admin widget: %s (plugin: %s)", widget.ID, pluginName)
	return nil
}

// RegisterPage registers a user-facing page
func (r *UIRegistry) RegisterPage(pluginName string, page *UIPage) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s", pluginName, page.ID)

	if _, exists := r.pages[key]; exists {
		return fmt.Errorf("page %s already registered by plugin %s", page.ID, pluginName)
	}

	page.PluginName = pluginName
	r.pages[key] = page

	log.Printf("[UI Registry] Registered page: %s (plugin: %s)", page.ID, pluginName)
	return nil
}

// RegisterAdminPage registers an admin page
func (r *UIRegistry) RegisterAdminPage(pluginName string, page *UIAdminPage) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s", pluginName, page.ID)

	if _, exists := r.adminPages[key]; exists {
		return fmt.Errorf("admin page %s already registered by plugin %s", page.ID, pluginName)
	}

	page.PluginName = pluginName
	r.adminPages[key] = page

	log.Printf("[UI Registry] Registered admin page: %s (plugin: %s)", page.ID, pluginName)
	return nil
}

// RegisterMenuItem registers a menu item
func (r *UIRegistry) RegisterMenuItem(pluginName string, item *UIMenuItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s", pluginName, item.ID)

	if _, exists := r.menuItems[key]; exists {
		return fmt.Errorf("menu item %s already registered by plugin %s", item.ID, pluginName)
	}

	item.PluginName = pluginName
	r.menuItems[key] = item

	log.Printf("[UI Registry] Registered menu item: %s (plugin: %s)", item.ID, pluginName)
	return nil
}

// UnregisterAll removes all UI components for a plugin
func (r *UIRegistry) UnregisterAll(pluginName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove widgets
	for key, widget := range r.widgets {
		if widget.PluginName == pluginName {
			delete(r.widgets, key)
		}
	}

	// Remove admin widgets
	for key, widget := range r.adminWidgets {
		if widget.PluginName == pluginName {
			delete(r.adminWidgets, key)
		}
	}

	// Remove pages
	for key, page := range r.pages {
		if page.PluginName == pluginName {
			delete(r.pages, key)
		}
	}

	// Remove admin pages
	for key, page := range r.adminPages {
		if page.PluginName == pluginName {
			delete(r.adminPages, key)
		}
	}

	// Remove menu items
	for key, item := range r.menuItems {
		if item.PluginName == pluginName {
			delete(r.menuItems, key)
		}
	}

	log.Printf("[UI Registry] Unregistered all UI components for plugin: %s", pluginName)
}

// GetWidgets returns all registered widgets
func (r *UIRegistry) GetWidgets() []*UIWidget {
	r.mu.RLock()
	defer r.mu.RUnlock()

	widgets := make([]*UIWidget, 0, len(r.widgets))
	for _, widget := range r.widgets {
		widgets = append(widgets, widget)
	}

	return widgets
}

// GetAdminWidgets returns all registered admin widgets
func (r *UIRegistry) GetAdminWidgets() []*UIWidget {
	r.mu.RLock()
	defer r.mu.RUnlock()

	widgets := make([]*UIWidget, 0, len(r.adminWidgets))
	for _, widget := range r.adminWidgets {
		widgets = append(widgets, widget)
	}

	return widgets
}

// GetPages returns all registered pages
func (r *UIRegistry) GetPages() []*UIPage {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pages := make([]*UIPage, 0, len(r.pages))
	for _, page := range r.pages {
		pages = append(pages, page)
	}

	return pages
}

// GetAdminPages returns all registered admin pages
func (r *UIRegistry) GetAdminPages() []*UIAdminPage {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pages := make([]*UIAdminPage, 0, len(r.adminPages))
	for _, page := range r.adminPages {
		pages = append(pages, page)
	}

	return pages
}

// GetMenuItems returns all registered menu items
func (r *UIRegistry) GetMenuItems() []*UIMenuItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]*UIMenuItem, 0, len(r.menuItems))
	for _, item := range r.menuItems {
		items = append(items, item)
	}

	return items
}

// PluginUI provides UI registration for plugins
type PluginUI struct {
	registry   *UIRegistry
	pluginName string
}

// NewPluginUI creates a new plugin UI instance
func NewPluginUI(registry *UIRegistry, pluginName string) *PluginUI {
	return &PluginUI{
		registry:   registry,
		pluginName: pluginName,
	}
}

// WidgetOptions contains options for registering a widget
type WidgetOptions struct {
	ID          string
	Title       string
	Component   string
	Position    string // "top", "sidebar", "bottom"
	Width       string // "full", "half", "third"
	Icon        string
	Permissions []string
}

// RegisterWidget registers a dashboard widget
func (pu *PluginUI) RegisterWidget(opts WidgetOptions) error {
	widget := &UIWidget{
		ID:          opts.ID,
		Title:       opts.Title,
		Component:   opts.Component,
		Position:    opts.Position,
		Width:       opts.Width,
		Icon:        opts.Icon,
		Permissions: opts.Permissions,
	}

	return pu.registry.RegisterWidget(pu.pluginName, widget)
}

// RegisterAdminWidget registers an admin dashboard widget
func (pu *PluginUI) RegisterAdminWidget(opts WidgetOptions) error {
	widget := &UIWidget{
		ID:          opts.ID,
		Title:       opts.Title,
		Component:   opts.Component,
		Position:    opts.Position,
		Width:       opts.Width,
		Icon:        opts.Icon,
		Permissions: opts.Permissions,
	}

	return pu.registry.RegisterAdminWidget(pu.pluginName, widget)
}

// PageOptions contains options for registering a page
type PageOptions struct {
	ID          string
	Title       string
	Path        string
	Component   string
	Icon        string
	MenuLabel   string
	Permissions []string
}

// RegisterPage registers a user-facing page
func (pu *PluginUI) RegisterPage(opts PageOptions) error {
	page := &UIPage{
		ID:          opts.ID,
		Title:       opts.Title,
		Path:        opts.Path,
		Component:   opts.Component,
		Icon:        opts.Icon,
		MenuLabel:   opts.MenuLabel,
		Permissions: opts.Permissions,
	}

	return pu.registry.RegisterPage(pu.pluginName, page)
}

// AdminPageOptions contains options for registering an admin page
type AdminPageOptions struct {
	ID          string
	Title       string
	Path        string
	Component   string
	Icon        string
	MenuLabel   string
	Permissions []string
	Order       int
}

// RegisterAdminPage registers an admin page
func (pu *PluginUI) RegisterAdminPage(opts AdminPageOptions) error {
	page := &UIAdminPage{
		ID:          opts.ID,
		Title:       opts.Title,
		Path:        opts.Path,
		Component:   opts.Component,
		Icon:        opts.Icon,
		MenuLabel:   opts.MenuLabel,
		Permissions: opts.Permissions,
		Order:       opts.Order,
	}

	return pu.registry.RegisterAdminPage(pu.pluginName, page)
}

// MenuItemOptions contains options for registering a menu item
type MenuItemOptions struct {
	ID          string
	Label       string
	Path        string
	Icon        string
	Component   string
	Order       int
	Permissions []string
}

// RegisterMenuItem registers a menu item
func (pu *PluginUI) RegisterMenuItem(opts MenuItemOptions) error {
	item := &UIMenuItem{
		ID:          opts.ID,
		Label:       opts.Label,
		Path:        opts.Path,
		Icon:        opts.Icon,
		Component:   opts.Component,
		Order:       opts.Order,
		Permissions: opts.Permissions,
	}

	return pu.registry.RegisterMenuItem(pu.pluginName, item)
}
