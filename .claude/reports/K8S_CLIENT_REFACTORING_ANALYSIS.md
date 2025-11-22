# k8sClient Usage Analysis for StreamSpace API

## Summary
Found **12 files** using `k8sClient` across the StreamSpace API codebase performing **50+ K8s operations** on multiple resource types.

---

## Detailed File Analysis

### 1. **api/cmd/main.go** (Initialization)
**File Path:** `/home/user/streamspace/api/cmd/main.go`

**Purpose:** Service initialization and dependency injection

**Handler Functions Using k8sClient:**
- `main()` - initializes k8sClient and injects into handlers
- `setupRoutes()` - configures routes with handlers using k8sClient

**K8s Operations:**
- None directly (initialization only)

**Resources:**
- Sessions (indirect - passed to handlers)
- Templates (indirect - passed to handlers)
- ApplicationInstalls (indirect - passed to handlers)
- Nodes (indirect - passed to handlers)

**Recommendation:** ✅ **STAY IN API** - Appropriate for initialization and dependency injection

**Details:**
```go
// Line 90: Initialize K8s client
k8sClient, err := k8s.NewClient()

// Line 238: Inject into API handler
apiHandler := api.NewHandler(database, k8sClient, connTracker, syncService, wsManager, quotaEnforcer)

// Line 242: Inject into activity handler
activityHandler := handlers.NewActivityHandler(k8sClient, activityTracker)

// Line 246: Inject into dashboard handler
dashboardHandler := handlers.NewDashboardHandler(database, k8sClient)

// Line 259: Inject into node handler
nodeHandler := handlers.NewNodeHandler(database, k8sClient)

// Line 274: Inject into application handler
applicationHandler := handlers.NewApplicationHandler(database, k8sClient, appNamespace)

// Line 123: Inject into websocket manager
wsManager := internalWebsocket.NewManager(database, k8sClient)

// Line 128: Inject into activity tracker
activityTracker := activity.NewTracker(k8sClient)

// Line 97: Inject into connection tracker
connTracker := tracker.NewConnectionTracker(database, k8sClient)
```

---

### 2. **api/internal/api/handlers.go** (Core Session/Template Management)
**File Path:** `/home/user/streamspace/api/internal/api/handlers.go`

**Purpose:** Main HTTP request handlers for session and template management

**Handler Functions Using k8sClient:**
- `ListSessions()` - List sessions by user or all sessions
- `GetSession()` - Get single session details
- `CreateSession()` - Create new session with quota check
- `UpdateSession()` - Update session state
- `DeleteSession()` - Delete session
- `UpdateSessionTags()` - Update session tags via dynamic client
- `ListSessionsByTags()` - List sessions filtered by tags
- `ListTemplates()` - List templates by category or all
- `GetTemplate()` - Get template details
- `CreateTemplate()` - Create template from manifest
- `DeleteTemplate()` - Delete template
- `UpdateTemplate()` (implied)
- `GetPods()` - Get pod list for quota calculation

**K8s Operations:**
- **Session CRD Operations:**
  - `ListSessions()` - READ (list)
  - `ListSessionsByUser()` - READ (list with filter)
  - `GetSession()` - READ (get single)
  - `CreateSession()` - CREATE
  - `UpdateSessionState()` - UPDATE (state field)
  - `DeleteSession()` - DELETE
  - Dynamic client: `Update()` on sessionGVR - UPDATE (tags field)

- **Template CRD Operations:**
  - `ListTemplates()` - READ (list)
  - `ListTemplatesByCategory()` - READ (list with filter)
  - `GetTemplate()` - READ (get single)
  - `CreateTemplate()` - CREATE
  - `DeleteTemplate()` - DELETE

- **Pod Operations:**
  - `GetPods()` - READ (list pods for quota calculation)

**Resources:**
- Sessions (primary)
- Templates (primary)
- Pods (quota calculation)

**Recommendation:** ⚠️ **CONSIDER MOVING TO CONTROLLER**
- Session lifecycle management (create/update/delete) should be controller responsibility
- Pod queries for quota checking could move to webhook admission controller
- Template management could stay in API (static resources)

**Critical Operations:**
```go
// Line 259-261: List sessions
sessions, err = h.k8sClient.ListSessionsByUser(ctx, h.namespace, userID)
sessions, err = h.k8sClient.ListSessions(ctx, h.namespace)

// Line 284: Get session
session, err := h.k8sClient.GetSession(ctx, h.namespace, sessionID)

// Line 364: Get template (validation)
template, err := h.k8sClient.GetTemplate(ctx, h.namespace, req.Template)

// Line 405: Get pods for quota calculation
podList, err := h.k8sClient.GetPods(ctx, h.namespace)

// Line 464: Create session
created, err := h.k8sClient.CreateSession(ctx, session)

// Line 500: Update session state
updated, err := h.k8sClient.UpdateSessionState(ctx, h.namespace, sessionID, req.State)

// Line 528: Delete session
if err := h.k8sClient.DeleteSession(ctx, h.namespace, sessionID)

// Line 657, 673: Update session tags (dynamic client)
obj, err := h.k8sClient.GetDynamicClient().Resource(sessionGVR).Namespace(h.namespace).Get(...)
_, err = h.k8sClient.GetDynamicClient().Resource(sessionGVR).Namespace(h.namespace).Update(...)

// Line 762-764: List templates
templates, err = h.k8sClient.ListTemplatesByCategory(ctx, h.namespace, category)
templates, err = h.k8sClient.ListTemplates(ctx, h.namespace)

// Line 884, 906, 921: Template operations
template, err := h.k8sClient.GetTemplate(ctx, h.namespace, templateID)
created, err := h.k8sClient.CreateTemplate(ctx, &template)
if err := h.k8sClient.DeleteTemplate(ctx, h.namespace, templateID)
```

---

### 3. **api/internal/api/stubs.go** (Cluster Management)
**File Path:** `/home/user/streamspace/api/internal/api/stubs.go`

**Purpose:** Generic cluster resource management (CRUD for any K8s resource type)

**Handler Functions Using k8sClient:**
- `ListNodes()` - List cluster nodes
- `ListPods()` - List pods in namespace
- `ListDeployments()` - List deployments
- `ListServices()` - List services
- `ListNamespaces()` - List namespaces
- `CreateResource()` - Create generic K8s resource
- `UpdateResource()` - Update generic K8s resource
- `DeleteResource()` - Delete generic K8s resource
- `GetPodLogs()` - Stream pod logs
- `GetConfig()` - Get platform configuration from ConfigMap
- `UpdateConfig()` - Update platform configuration in ConfigMap
- `GetMetrics()` - Get resource metrics

**K8s Operations:**
- **Node Operations:**
  - `GetNodes()` - READ (list all nodes)
  - `GetNode()` - READ (single node details)

- **Pod Operations:**
  - `GetPods()` - READ (list pods)
  - `GetClientset().CoreV1().Pods().GetLogs()` - READ (pod logs)

- **Deployment Operations:**
  - `GetClientset().AppsV1().Deployments().List()` - READ (list deployments)

- **Service Operations:**
  - `GetServices()` - READ (list services)

- **Namespace Operations:**
  - `GetNamespaces()` - READ (list namespaces)

- **Dynamic Resource Operations:**
  - `GetDynamicClient().Resource(gvr).Create()` - CREATE
  - `GetDynamicClient().Resource(gvr).Update()` - UPDATE
  - `GetDynamicClient().Resource(gvr).Delete()` - DELETE

- **ConfigMap Operations:**
  - `GetClientset().CoreV1().ConfigMaps().Get()` - READ
  - `GetClientset().CoreV1().ConfigMaps().Create()` - CREATE
  - `GetClientset().CoreV1().ConfigMaps().Update()` - UPDATE

**Resources:**
- Nodes
- Pods
- Deployments
- Services
- Namespaces
- ConfigMaps
- Generic K8s resources (via dynamic client)

**Recommendation:** ⚠️ **CONSIDER MOVING TO CONTROLLER**
- Node management (cordon, drain, taint) - belongs in controller
- Dynamic resource creation/update/delete - should be admission webhook or CRD validation
- Pod log streaming could stay in API (read-only, real-time)
- ConfigMap management (application configuration) - belongs in controller or config service

**Details:**
```go
// Nodes
nodeList, err := h.k8sClient.GetNodes(c.Request.Context())
nodes, err = h.k8sClient.GetNodes(ctx)

// Pods
pods, err := h.k8sClient.GetPods(c.Request.Context(), namespace)
req := h.k8sClient.GetClientset().CoreV1().Pods(namespace).GetLogs(podName, opts)

// Deployments
deployments, err := h.k8sClient.GetClientset().AppsV1().Deployments(namespace).List(c.Request.Context(), metav1.ListOptions{})

// Services
services, err := h.k8sClient.GetServices(c.Request.Context(), namespace)

// Namespaces
namespaces, err := h.k8sClient.GetNamespaces(c.Request.Context())

// Dynamic resources
created, err := h.k8sClient.GetDynamicClient().Resource(gvr).Namespace(namespace).Create(...)
updated, err := h.k8sClient.GetDynamicClient().Resource(gvr).Namespace(namespace).Update(...)
err = h.k8sClient.GetDynamicClient().Resource(gvr).Namespace(namespace).Delete(...)

// ConfigMaps
configMap, err := h.k8sClient.GetClientset().CoreV1().ConfigMaps(h.namespace).Get(...)
_, err = h.k8sClient.GetClientset().CoreV1().ConfigMaps(h.namespace).Create(...)
_, err = h.k8sClient.GetClientset().CoreV1().ConfigMaps(h.namespace).Update(...)
```

---

### 4. **api/internal/handlers/applications.go** (Application Installation)
**File Path:** `/home/user/streamspace/api/internal/handlers/applications.go`

**Purpose:** Installed application management and lifecycle

**Handler Functions Using k8sClient:**
- `InstallApplication()` - Install new application from catalog

**K8s Operations:**
- **ApplicationInstall CRD:**
  - `CreateApplicationInstall()` - CREATE

**Resources:**
- ApplicationInstall (CRD)

**Recommendation:** ✅ **STAY IN API** - Application installation is an administrative operation
- API initiates installation request
- Controller watches ApplicationInstall and creates Template
- Proper separation of concerns

**Details:**
```go
// Line 221: Create ApplicationInstall CRD
_, err = h.k8sClient.CreateApplicationInstall(ctx, appInstall)

// Step shows handling of errors:
// - "already exists" - continues with DB record
// - "not find the requested resource" - logs warning but continues
// - other errors - returns HTTP 500
```

---

### 5. **api/internal/handlers/nodes.go** (Cluster Node Management)
**File Path:** `/home/user/streamspace/api/internal/handlers/nodes.go`

**Purpose:** Administrator node management (labels, taints, cordon, drain)

**Handler Functions Using k8sClient:**
- `ListNodes()` - List all cluster nodes
- `GetNode()` - Get single node details
- `GetClusterStats()` - Aggregate cluster statistics
- `AddNodeLabel()` - Add label to node
- `RemoveNodeLabel()` - Remove label from node
- `AddNodeTaint()` - Add taint to node
- `RemoveNodeTaint()` - Remove taint from node
- `CordonNode()` - Mark node as unschedulable
- `UncordonNode()` - Mark node as schedulable
- `DrainNode()` - Evict all pods from node

**K8s Operations:**
- **Node Operations:**
  - `GetNodes()` - READ (list all nodes)
  - `GetNode()` - READ (single node)
  - `PatchNode()` - UPDATE (labels and taints)
  - `UpdateNodeTaints()` - UPDATE (taints specifically)
  - `CordonNode()` - UPDATE (unschedulable flag)
  - `UncordonNode()` - UPDATE (unschedulable flag)
  - `DrainNode()` - DELETE (evict pods)

**Resources:**
- Nodes (primary)
- Pods (implicit - evicted during drain)

**Recommendation:** ⚠️ **CONSIDER MOVING TO CONTROLLER**
- Node operations are cluster infrastructure management
- Should be handled by cluster operator controller
- Could be triggered by custom CRD (NodeMaintenanceRequest)
- API could remain as read-only endpoints for monitoring

**Details:**
```go
// List and Get
nodeList, err := h.k8sClient.GetNodes(ctx)
node, err := h.k8sClient.GetNode(ctx, nodeName)

// Patch (labels and taints)
patchData := fmt.Sprintf(`{"metadata":{"labels":{"%s":"%s"}}}`, req.Key, req.Value)
if err := h.k8sClient.PatchNode(ctx, nodeName, []byte(patchData))

// Cordon/Uncordon
if err := h.k8sClient.CordonNode(ctx, nodeName)
if err := h.k8sClient.UncordonNode(ctx, nodeName)

// Drain
if err := h.k8sClient.DrainNode(ctx, nodeName, req.GracePeriodSeconds)
```

---

### 6. **api/internal/handlers/dashboard.go** (Dashboard Statistics)
**File Path:** `/home/user/streamspace/api/internal/handlers/dashboard.go`

**Purpose:** Platform statistics and dashboard metrics

**Handler Functions Using k8sClient:**
- `GetPlatformStats()` - Get overall platform statistics

**K8s Operations:**
- **Template Operations:**
  - `ListTemplates()` - READ (list templates for count)

**Resources:**
- Templates (for template count metric)

**Recommendation:** ✅ **STAY IN API** - Read-only dashboard queries belong in API
- No state changes
- Real-time metric aggregation
- Appropriate for API tier

---

### 7. **api/internal/handlers/activity.go** (Session Activity Tracking)
**File Path:** `/home/user/streamspace/api/internal/handlers/activity.go`

**Purpose:** Session activity heartbeat recording

**Handler Functions Using k8sClient:**
- `RecordHeartbeat()` - Record session activity (delegates to activity.Tracker)
- `GetActivity()` - Get session activity status

**K8s Operations:**
- Indirectly called via activity.Tracker:
  - `GetSession()` - READ (session for activity status)
  - `UpdateSessionStatus()` - UPDATE (lastActivity timestamp)

**Resources:**
- Sessions (activity status)

**Recommendation:** ✅ **STAY IN API** - Activity heartbeats must be low-latency responses
- Real-time heartbeat updates
- Cannot defer to controller (latency unacceptable)
- API layer appropriate for this

---

### 8. **api/internal/activity/tracker.go** (Idle Detection)
**File Path:** `/home/user/streamspace/api/internal/activity/tracker.go`

**Purpose:** Background idle session monitoring and auto-hibernation

**Handler Functions Using k8sClient:**
- `UpdateSessionActivity()` - Update lastActivity timestamp
- `GetActivityStatus()` - Calculate idle state
- `StartIdleMonitor()` - Background monitor (periodic)
- `hibernateIdleSessions()` - Auto-hibernate idle sessions

**K8s Operations:**
- `GetSession()` - READ (check idle status)
- `UpdateSessionStatus()` - UPDATE (lastActivity)
- `ListSessions()` - READ (list all for idle check)
- `UpdateSession()` - UPDATE (state to "hibernated")

**Resources:**
- Sessions (idle monitoring and hibernation)

**Recommendation:** ⚠️ **MOVE TO CONTROLLER**
- Idle detection is controller responsibility
- Session state transitions belong in controller
- Should implement custom controller with hibernation logic
- Activity tracking could stay in API for heartbeat updates

---

### 9. **api/internal/tracker/tracker.go** (Connection Tracking)
**File Path:** `/home/user/streamspace/api/internal/tracker/tracker.go`

**Purpose:** Active connection monitoring and auto-start/hibernate logic

**Handler Functions Using k8sClient:**
- `autoStartHibernatedSession()` - Start hibernated session when connection arrives
- `autoHibernateIdleSessions()` - Hibernate sessions with no connections
- Background goroutine: `Start()` - periodic checks

**K8s Operations:**
- `GetSession()` - READ (check session state)
- `UpdateSessionState()` - UPDATE (state to "running" or "hibernated")

**Resources:**
- Sessions (state management)

**Recommendation:** ⚠️ **MOVE TO CONTROLLER**
- Session state transitions must be in controller
- Connection tracking could stay in API
- Controller should implement auto-start/hibernate logic
- API should track connections and update controller via CRD/webhook

---

### 10. **api/internal/websocket/handlers.go** (Real-time Updates)
**File Path:** `/home/user/streamspace/api/internal/websocket/handlers.go`

**Purpose:** WebSocket streaming of sessions and pod logs

**Handler Functions Using k8sClient:**
- `broadcastSessionUpdates()` - Periodic session broadcast
- `broadcastMetrics()` - Periodic metrics broadcast
- `LogsWebSocket()` - Stream pod logs via WebSocket

**K8s Operations:**
- `ListSessions()` - READ (list sessions for broadcast)
- `GetClientset().CoreV1().Pods().GetLogs()` - READ (pod logs)

**Resources:**
- Sessions (read-only broadcast)
- Pods (log streaming)

**Recommendation:** ✅ **STAY IN API** - Real-time WebSocket updates belong in API
- Read-only operations
- Real-time response requirement
- Low-latency streaming

---

### 11. **api/internal/middleware/quota.go** (Quota Enforcement)
**File Path:** `/home/user/streamspace/api/internal/middleware/quota.go`

**Purpose:** Quota middleware integration (minimal k8sClient usage)

**Handler Functions Using k8sClient:**
- None directly in middleware

**K8s Operations:**
- None (middleware just validates, handlers use k8sClient)

**Recommendation:** ✅ **STAY IN API** - Quota enforcement is API responsibility

---

### 12. **api/internal/api/handlers_test.go** (Unit Tests)
**File Path:** `/home/user/streamspace/api/internal/api/handlers_test.go`

**Purpose:** Handler tests (mocked k8sClient)

**Handler Functions Using k8sClient:**
- Mock usage in test setup

**K8s Operations:**
- Mock operations for testing

**Recommendation:** N/A - Test file, no migration needed

---

## Summary Table

| File | Functions Count | K8s Operations | Resources | Move to Controller? | Priority |
|------|-----------------|-----------------|-----------|---------------------|----------|
| api/cmd/main.go | 2 | 0 (init only) | Multiple | No | N/A |
| api/internal/api/handlers.go | 13+ | 15+ (CRUD) | Sessions, Templates, Pods | YES - Critical | HIGH |
| api/internal/api/stubs.go | 12 | 20+ (CRUD) | Nodes, Pods, Services, ConfigMaps, Generic | YES - Some ops | HIGH |
| api/internal/handlers/applications.go | 1 | 1 (CREATE) | ApplicationInstall | No - API appropriate | MED |
| api/internal/handlers/nodes.go | 9 | 9 (UPDATE) | Nodes | YES - Infrastructure | MED |
| api/internal/handlers/dashboard.go | 1 | 1 (READ) | Templates | No - Read-only | LOW |
| api/internal/handlers/activity.go | 2 | 2 (READ/UPDATE) | Sessions | No - Real-time | MED |
| api/internal/activity/tracker.go | 4 | 4 (READ/UPDATE) | Sessions | YES - Logic | HIGH |
| api/internal/tracker/tracker.go | 2 | 2 (READ/UPDATE) | Sessions | YES - State mgmt | HIGH |
| api/internal/websocket/handlers.go | 3 | 2 (READ) | Sessions, Pods | No - Streaming | LOW |
| api/internal/middleware/quota.go | - | 0 | - | N/A | N/A |
| api/internal/api/handlers_test.go | - | Mock | - | N/A | N/A |

---

## Refactoring Recommendations

### HIGH PRIORITY - Move to Controller
1. **Session lifecycle management** (api/internal/api/handlers.go)
   - CreateSession → Controller creation logic
   - UpdateSessionState → Controller state machine
   - DeleteSession → Controller cleanup
   - Keep GetSession/ListSessions in API

2. **Idle detection & hibernation** (api/internal/activity/tracker.go)
   - Implement controller reconciler for idle sessions
   - API keeps heartbeat update endpoint (low-latency)
   - Controller monitors lastActivity timestamp

3. **Connection-based auto-start** (api/internal/tracker/tracker.go)
   - Move auto-start logic to controller
   - API tracks connections, controller manages state
   - Consider webhook for connection events

### MEDIUM PRIORITY - Evaluate
1. **Node management** (api/internal/handlers/nodes.go)
   - Consider NodeMaintenanceRequest CRD pattern
   - Keep read-only endpoints in API
   - Move state-changing operations to controller

2. **Application installation** (api/internal/handlers/applications.go)
   - Current pattern is good (API triggers, Controller executes)
   - Monitor for patterns

### KEEP IN API
1. Dashboard queries (read-only aggregation)
2. WebSocket streaming (real-time, read-only)
3. Activity heartbeats (must be low-latency)
4. Application installation triggers (initiating operations)
5. Template list/get (read-only catalog)

---

## K8s Operations Summary

### By Operation Type
| Operation | Count | Files | Resources |
|-----------|-------|-------|-----------|
| CREATE | 8 | handlers.go, applications.go, stubs.go | Sessions, Templates, ApplicationInstall, ConfigMap, Generic |
| READ (List) | 20 | handlers.go, stubs.go, dashboard.go, activity.go, tracker.go, websocket.go | Sessions, Templates, Nodes, Pods, Deployments, Services, Namespaces |
| READ (Get) | 15 | handlers.go, stubs.go, activity.go, tracker.go, nodes.go | Sessions, Templates, Nodes, Pods, ConfigMaps |
| UPDATE | 18 | handlers.go, stubs.go, activity.go, tracker.go, nodes.go | Sessions, Templates, ConfigMaps, Nodes, Generic |
| DELETE | 6 | handlers.go, stubs.go, nodes.go | Sessions, Templates, Generic Resources, Pods (evict) |
| PATCH | 3 | nodes.go | Nodes (labels, taints) |
| STREAM | 1 | websocket.go | Pods (logs) |

### By Resource Type
| Resource | Operations | Files | Current Tier | Recommended |
|----------|-----------|-------|--------------|-------------|
| Session | CRUD + Update State | handlers.go, activity.go, tracker.go | API | Controller |
| Template | CRUD | handlers.go, stubs.go, dashboard.go | API | Hybrid (API read, Controller write) |
| ApplicationInstall | CREATE | applications.go | API | Keep in API (trigger) |
| Node | Get, Patch, Cordon, Drain | nodes.go, stubs.go | API | Controller |
| Pod | Get, List, Logs, Evict | handlers.go, stubs.go, websocket.go | API | Hybrid (keep streaming/query, move eviction) |
| ConfigMap | Get, Create, Update | stubs.go | API | Controller |
| Deployment | List | stubs.go | API | Keep in API (monitoring) |
| Service | List | stubs.go | API | Keep in API (monitoring) |
| Namespace | List | stubs.go | API | Keep in API (monitoring) |
| Generic Resources | CRUD | stubs.go | API | Controller (via webhooks) |

