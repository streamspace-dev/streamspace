// Package plugins provides the plugin system for StreamSpace API.
//
// The ui_registry component enables plugins to register custom UI components
// that are dynamically integrated into the React frontend. This allows plugins
// to extend the user interface without modifying core UI code.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────┐
//	│                React Frontend (Browser)                     │
//	│  Fetches UI metadata from /api/plugins/ui/components       │
//	└──────────────────────────┬──────────────────────────────────┘
//	                           │ HTTP API
//	                           ↓
//	┌─────────────────────────────────────────────────────────────┐
//	│                      UIRegistry                             │
//	│  - Widgets: Dashboard cards (session stats, alerts)        │
//	│  - Pages: Full pages (/plugins/slack/messages)             │
//	│  - AdminPages: Admin panel pages (/admin/plugins/slack)    │
//	│  - MenuItems: Navigation menu entries                      │
//	│  - AdminWidgets: Admin dashboard widgets                   │
//	└──────────────────────────┬──────────────────────────────────┘
//	                           │ Registered by
//	                           ↓
//	┌─────────────────────────────────────────────────────────────┐
//	│                     Plugin OnLoad()                         │
//	│  ui.RegisterWidget({title: "Slack Stats", ...})            │
//	│  ui.RegisterPage({path: "/messages", ...})                 │
//	│  ui.RegisterMenuItem({label: "Slack", ...})                │
//	└─────────────────────────────────────────────────────────────┘
//
// UI Component Types:
//
//  1. Widgets: Dashboard cards on user home page
//     - Position: top, sidebar, bottom
//     - Width: full, half, third
//     - Example: "Session Activity", "Quota Usage"
//
//  2. Pages: Full user-facing pages
//     - Custom routes under /plugins/{name}/
//     - Example: /plugins/slack/messages
//
//  3. AdminPages: Admin panel pages
//     - Custom routes under /admin/plugins/{name}/
//     - Example: /admin/plugins/slack/settings
//
//  4. MenuItems: Navigation menu entries
//     - Appear in main navigation menu
//     - Link to plugin pages or external URLs
//
//  5. AdminWidgets: Admin dashboard widgets
//     - Similar to widgets but for admin dashboard
//     - Example: "Plugin Health", "License Status"
//
// Component Lifecycle:
//  1. Plugin calls ui.RegisterWidget() during OnLoad()
//  2. UIRegistry stores component metadata
//  3. Frontend calls /api/plugins/ui/components
//  4. Registry returns all component metadata as JSON
//  5. React renders components dynamically
//  6. Plugin unload removes components via UnregisterAll()
//
// Dynamic UI Loading:
//
// The frontend fetches component metadata and renders them dynamically:
//
//	// Frontend code
//	fetch('/api/plugins/ui/components')
//	  .then(res => res.json())
//	  .then(data => {
//	    renderWidgets(data.widgets)
//	    registerPages(data.pages)
//	    updateMenu(data.menuItems)
//	  })
//
// Component Rendering:
//
//	Plugins can provide:
//	  - Component name (React component string)
//	  - Inline HTML/React JSX
//	  - URL to external React component bundle
//
//	The frontend uses dynamic import() to load plugin components.
//
// Thread Safety:
//
// The registry uses sync.RWMutex for thread-safe concurrent access:
//   - Register methods: Exclusive lock (write)
//   - Get methods: Shared lock (read)
//   - Safe for plugins to register during parallel OnLoad() calls
//
// Permissions:
//
// UI components can declare required permissions. The frontend queries
// user permissions and only renders components the user can access:
//
//	ui.RegisterWidget(WidgetOptions{
//	    Title: "Admin Stats",
//	    Permissions: []string{"admin.read"},  // Only visible to admins
//	})
//
// Component Cleanup:
//
// When a plugin is unloaded:
//   - UnregisterAll(pluginName) removes all UI components
//   - Frontend polls for updates and removes components
//   - Prevents orphaned UI elements from unloaded plugins
//
// Performance:
//   - Registration: O(1) map insertion
//   - Lookup: O(1) map access
//   - GetAll operations: O(n) iteration
//   - Memory: ~300 bytes per component registration
//
// Future Enhancements:
//   - Hot reloading without frontend refresh
//   - Component versioning
//   - Server-side rendering (SSR) for plugin UIs
//   - Plugin UI theming and customization
//   - WebSocket-based real-time component updates
package plugins

import (
	"fmt"
	"log"
	"sync"
)

// UIRegistry manages plugin UI component registrations.
//
// The registry provides centralized management of all plugin-contributed UI
// components, enabling dynamic frontend integration without core code changes.
//
// Key responsibilities:
//   - Store UI component registrations with plugin attribution
//   - Support multiple component types (widgets, pages, menus)
//   - Prevent component ID conflicts between plugins
//   - Provide thread-safe concurrent access
//   - Support bulk cleanup on plugin unload
//
// Registry Structure:
//
//	widgets:      map[string]*UIWidget         // User dashboard widgets
//	pages:        map[string]*UIPage           // User-facing pages
//	adminPages:   map[string]*UIAdminPage      // Admin panel pages
//	menuItems:    map[string]*UIMenuItem       // Navigation menu items
//	adminWidgets: map[string]*UIWidget         // Admin dashboard widgets
//
//	Map key format: "{pluginName}:{componentID}"
//	Example: "slack:widget-stats"
//
// Concurrency Model:
//
//	Register methods: Write lock (exclusive)
//	Get methods: Read lock (shared)
//	UnregisterAll: Write lock (exclusive)
//	Multiple plugins can query concurrently
//	Registration is serialized to prevent conflicts
type UIRegistry struct {
	// widgets stores user dashboard widgets.
	// Map key: "{pluginName}:{widgetID}"
	widgets map[string]*UIWidget

	// pages stores user-facing pages.
	// Map key: "{pluginName}:{pageID}"
	pages map[string]*UIPage

	// adminPages stores admin panel pages.
	// Map key: "{pluginName}:{pageID}"
	adminPages map[string]*UIAdminPage

	// menuItems stores navigation menu items.
	// Map key: "{pluginName}:{itemID}"
	menuItems map[string]*UIMenuItem

	// adminWidgets stores admin dashboard widgets.
	// Map key: "{pluginName}:{widgetID}"
	adminWidgets map[string]*UIWidget

	// mu protects concurrent access to all component maps.
	// Read operations (Get*) use RLock.
	// Write operations (Register*, Unregister*) use Lock.
	mu sync.RWMutex
}

// UIWidget represents a dashboard widget.
//
// Widgets are cards displayed on the user's home dashboard. They can show
// real-time data, quick actions, or status information.
//
// Layout:
//   - Position: Where on the dashboard (top, sidebar, bottom)
//   - Width: How much horizontal space (full=100%, half=50%, third=33%)
//
// Example widgets:
//   - "Session Activity": Recent session usage
//   - "Quota Status": Resource usage bars
//   - "Quick Actions": Buttons to create sessions
//
// Example:
//
//	&UIWidget{
//	    ID:          "session-stats",
//	    Title:       "Session Statistics",
//	    Component:   "SessionStatsWidget",  // React component name
//	    Position:    "top",
//	    Width:       "half",
//	    Icon:        "chart-line",
//	    Permissions: []string{"sessions.read"},
//	}
type UIWidget struct {
	// PluginName identifies which plugin registered this widget.
	// Set automatically by the registry.
	PluginName string

	// ID is a unique identifier for this widget within the plugin.
	// Format: kebab-case (e.g., "session-stats")
	ID string

	// Title is displayed as the widget header.
	// Example: "Session Statistics"
	Title string

	// Component is the React component name or bundle URL.
	// Can be:
	//   - Component name: "SessionStatsWidget"
	//   - Bundle URL: "/plugins/slack/widget.js"
	Component string

	// Position determines vertical placement on the dashboard.
	// Values: "top", "sidebar", "bottom"
	Position string

	// Width determines horizontal size.
	// Values: "full" (100%), "half" (50%), "third" (33%)
	Width string

	// Icon is the icon name from the icon library.
	// Example: "chart-line", "bell", "users"
	Icon string

	// Permissions lists required permissions to view this widget.
	// Frontend checks user permissions before rendering.
	// Empty = visible to all users.
	Permissions []string
}

// UIPage represents a user-facing page.
//
// Pages are full-page components rendered at custom routes. They provide
// complete plugin-specific interfaces within the main application.
//
// URL Format:
//
//	/plugins/{pluginName}/{path}
//	Example: /plugins/slack/messages
//
// Navigation:
//
//	Pages can appear in navigation menus if MenuLabel is set.
//	Otherwise, they're accessible only by direct URL.
//
// Example:
//
//	&UIPage{
//	    ID:          "messages",
//	    Title:       "Slack Messages",
//	    Path:        "/messages",  // Results in /plugins/slack/messages
//	    Component:   "SlackMessagesPage",
//	    Icon:        "comment",
//	    MenuLabel:   "Messages",  // Appears in menu
//	    Permissions: []string{"plugin.slack.read"},
//	}
type UIPage struct {
	// PluginName identifies which plugin registered this page.
	// Set automatically by the registry.
	PluginName string

	// ID is a unique identifier for this page within the plugin.
	ID string

	// Title is the page title shown in browser tab and header.
	Title string

	// Path is the route path relative to /plugins/{pluginName}/.
	// Example: "/messages" becomes "/plugins/slack/messages"
	Path string

	// Component is the React component name or bundle URL.
	Component string

	// Icon is the icon shown in menus and browser tab.
	Icon string

	// MenuLabel is the text shown in navigation menus.
	// If empty, page is not added to menus (direct URL only).
	MenuLabel string

	// Permissions lists required permissions to access this page.
	// Frontend enforces access control before rendering.
	Permissions []string
}

// UIAdminPage represents an admin panel page.
//
// Admin pages are similar to regular pages but appear in the admin panel
// and typically require admin permissions.
//
// URL Format:
//
//	/admin/plugins/{pluginName}/{path}
//	Example: /admin/plugins/slack/settings
//
// Menu Ordering:
//
//	Admin pages appear in the admin menu sorted by Order field.
//	Lower numbers appear first.
//
// Example:
//
//	&UIAdminPage{
//	    ID:          "settings",
//	    Title:       "Slack Settings",
//	    Path:        "/settings",
//	    Component:   "SlackAdminSettings",
//	    Icon:        "cog",
//	    MenuLabel:   "Slack",
//	    Permissions: []string{"admin.plugins.manage"},
//	    Order:       100,
//	}
type UIAdminPage struct {
	// PluginName identifies which plugin registered this page.
	PluginName string

	// ID is a unique identifier for this page within the plugin.
	ID string

	// Title is the page title.
	Title string

	// Path is the route path relative to /admin/plugins/{pluginName}/.
	Path string

	// Component is the React component name or bundle URL.
	Component string

	// Icon is the icon shown in admin menu.
	Icon string

	// MenuLabel is the text shown in admin navigation menu.
	MenuLabel string

	// Permissions lists required permissions (typically admin permissions).
	Permissions []string

	// Order determines position in admin menu (lower = earlier).
	// Typical range: 0-1000
	Order int
}

// UIMenuItem represents a navigation menu item.
//
// Menu items appear in the main navigation menu and can link to:
//   - Plugin pages
//   - External URLs
//   - Custom components
//
// Menu Ordering:
//
//	Items are sorted by Order field. Lower numbers appear first.
//	Standard menu items use Order 100-900.
//	Plugin items typically use Order 1000+.
//
// Example:
//
//	&UIMenuItem{
//	    ID:          "slack-menu",
//	    Label:       "Slack",
//	    Path:        "/plugins/slack/messages",
//	    Icon:        "slack",
//	    Order:       1000,
//	    Permissions: []string{"plugin.slack.read"},
//	}
type UIMenuItem struct {
	// PluginName identifies which plugin registered this item.
	PluginName string

	// ID is a unique identifier for this item within the plugin.
	ID string

	// Label is the text displayed in the menu.
	Label string

	// Path is the URL to navigate to when clicked.
	// Can be:
	//   - Internal: "/plugins/slack/messages"
	//   - External: "https://slack.com"
	Path string

	// Icon is the icon shown next to the label.
	Icon string

	// Component is an optional custom React component for the menu item.
	// If empty, standard menu item rendering is used.
	Component string

	// Order determines position in menu (lower = earlier).
	// Recommended: 1000+ for plugin items.
	Order int

	// Permissions lists required permissions to see this menu item.
	Permissions []string
}

// NewUIRegistry creates a new UI registry.
//
// Returns an initialized registry ready to accept plugin UI component registrations.
func NewUIRegistry() *UIRegistry {
	return &UIRegistry{
		widgets:      make(map[string]*UIWidget),
		pages:        make(map[string]*UIPage),
		adminPages:   make(map[string]*UIAdminPage),
		menuItems:    make(map[string]*UIMenuItem),
		adminWidgets: make(map[string]*UIWidget),
	}
}

// RegisterWidget registers a user dashboard widget.
//
// Stores widget metadata for display on the user's home dashboard. Frontend
// fetches registered widgets via API and renders them dynamically.
//
// Parameters:
//   - pluginName: Name of the plugin registering the widget
//   - widget: Widget configuration (title, component, position, width)
//
// Returns:
//   - error: Conflict error if widget ID already registered, nil on success
//
// Thread Safety: Acquires exclusive write lock.
//
// Example:
//
//	err := registry.RegisterWidget("slack", &UIWidget{
//	    ID: "stats", Title: "Slack Stats", Position: "top", Width: "half",
//	})
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

// RegisterAdminWidget registers an admin dashboard widget.
//
// Similar to RegisterWidget but for admin dashboard. Admin widgets typically
// display platform-wide metrics, plugin health, or administrative quick actions.
//
// Thread Safety: Acquires exclusive write lock.
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

// RegisterPage registers a user-facing page.
//
// Registers a full-page component accessible at /plugins/{pluginName}/{path}.
// Pages can optionally appear in navigation menus if MenuLabel is set.
//
// Thread Safety: Acquires exclusive write lock.
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

// RegisterAdminPage registers an admin panel page.
//
// Registers an admin page accessible at /admin/plugins/{pluginName}/{path}.
// Admin pages appear in admin navigation menu sorted by Order field.
//
// Thread Safety: Acquires exclusive write lock.
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

// RegisterMenuItem registers a navigation menu item.
//
// Menu items appear in the main navigation menu. They can link to plugin pages,
// external URLs, or use custom components. Items are sorted by Order field.
//
// Thread Safety: Acquires exclusive write lock.
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

// UnregisterAll removes all UI components for a plugin.
//
// Called during plugin unload to clean up all widgets, pages, admin pages,
// menu items, and admin widgets registered by the plugin.
//
// Thread Safety: Acquires exclusive write lock.
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

// GetWidgets returns all registered user dashboard widgets.
//
// Returns a snapshot of all widgets for the user home dashboard. Frontend
// fetches this to render widgets dynamically.
//
// Thread Safety: Acquires shared read lock.
//
// Returns: Slice of all registered widgets (copy, safe to modify)
func (r *UIRegistry) GetWidgets() []*UIWidget {
	r.mu.RLock()
	defer r.mu.RUnlock()

	widgets := make([]*UIWidget, 0, len(r.widgets))
	for _, widget := range r.widgets {
		widgets = append(widgets, widget)
	}

	return widgets
}

// GetAdminWidgets returns all registered admin dashboard widgets.
//
// Returns a snapshot of all widgets for the admin dashboard. Admin UI
// fetches this to render admin-specific widgets.
//
// Thread Safety: Acquires shared read lock.
func (r *UIRegistry) GetAdminWidgets() []*UIWidget {
	r.mu.RLock()
	defer r.mu.RUnlock()

	widgets := make([]*UIWidget, 0, len(r.adminWidgets))
	for _, widget := range r.adminWidgets {
		widgets = append(widgets, widget)
	}

	return widgets
}

// GetPages returns all registered user-facing pages.
//
// Returns a snapshot of all pages. Frontend uses this to register routes
// and populate navigation menus.
//
// Thread Safety: Acquires shared read lock.
func (r *UIRegistry) GetPages() []*UIPage {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pages := make([]*UIPage, 0, len(r.pages))
	for _, page := range r.pages {
		pages = append(pages, page)
	}

	return pages
}

// GetAdminPages returns all registered admin pages.
//
// Returns a snapshot of all admin pages. Admin UI uses this to register
// routes and populate admin navigation menu.
//
// Thread Safety: Acquires shared read lock.
func (r *UIRegistry) GetAdminPages() []*UIAdminPage {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pages := make([]*UIAdminPage, 0, len(r.adminPages))
	for _, page := range r.adminPages {
		pages = append(pages, page)
	}

	return pages
}

// GetMenuItems returns all registered navigation menu items.
//
// Returns a snapshot of all menu items. Frontend uses this to populate
// the main navigation menu, sorted by Order field.
//
// Thread Safety: Acquires shared read lock.
func (r *UIRegistry) GetMenuItems() []*UIMenuItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]*UIMenuItem, 0, len(r.menuItems))
	for _, item := range r.menuItems {
		items = append(items, item)
	}

	return items
}

// PluginUI provides UI registration interface for plugins.
//
// This is the plugin-facing API that abstracts the underlying UIRegistry.
// Each plugin receives a PluginUI instance pre-configured with its name,
// ensuring automatic registration attribution.
//
// Example Usage in Plugin:
//
//	func (p *SlackPlugin) OnLoad(ctx *PluginContext) error {
//	    // Register a widget
//	    ctx.UI.RegisterWidget(WidgetOptions{
//	        ID: "stats", Title: "Slack Stats", Position: "top", Width: "half",
//	    })
//	    // Register a page
//	    ctx.UI.RegisterPage(PageOptions{
//	        ID: "messages", Title: "Messages", Path: "/messages",
//	    })
//	    return nil
//	}
type PluginUI struct {
	// registry is the global UI registry.
	registry *UIRegistry

	// pluginName is the name of the plugin this UI instance serves.
	pluginName string
}

// NewPluginUI creates a new plugin UI instance.
//
// Creates a scoped UI interface for a specific plugin. Called by the plugin
// runtime during initialization, not by plugins directly.
func NewPluginUI(registry *UIRegistry, pluginName string) *PluginUI {
	return &PluginUI{
		registry:   registry,
		pluginName: pluginName,
	}
}

// WidgetOptions contains options for registering a widget.
//
// Fields:
//   - ID: Unique widget identifier within plugin
//   - Title: Widget header text
//   - Component: React component name or bundle URL
//   - Position: Dashboard placement ("top", "sidebar", "bottom")
//   - Width: Horizontal size ("full", "half", "third")
//   - Icon: Icon name
//   - Permissions: Required permissions to view
type WidgetOptions struct {
	ID          string
	Title       string
	Component   string
	Position    string
	Width       string
	Icon        string
	Permissions []string
}

// RegisterWidget registers a user dashboard widget.
//
// Registers a widget for display on the user home dashboard.
//
// Returns: error if widget ID conflicts, nil on success
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

// RegisterAdminWidget registers an admin dashboard widget.
//
// Similar to RegisterWidget but for admin dashboard.
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

// PageOptions contains options for registering a page.
//
// Fields:
//   - ID, Title, Path, Component, Icon: Page metadata
//   - MenuLabel: If set, page appears in navigation menu
//   - Permissions: Required permissions to access
type PageOptions struct {
	ID          string
	Title       string
	Path        string
	Component   string
	Icon        string
	MenuLabel   string
	Permissions []string
}

// RegisterPage registers a user-facing page at /plugins/{name}/{path}.
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

// AdminPageOptions contains options for registering an admin page.
//
// Fields:
//   - Order: Position in admin menu (lower = earlier)
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

// RegisterAdminPage registers an admin page at /admin/plugins/{name}/{path}.
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

// MenuItemOptions contains options for registering a menu item.
//
// Fields:
//   - Label: Menu text
//   - Path: URL to navigate to
//   - Order: Position in menu (lower = earlier, use 1000+ for plugins)
type MenuItemOptions struct {
	ID          string
	Label       string
	Path        string
	Icon        string
	Component   string
	Order       int
	Permissions []string
}

// RegisterMenuItem registers a navigation menu item.
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
