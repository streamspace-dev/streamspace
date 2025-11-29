import axios, { AxiosInstance, AxiosError } from 'axios';
import { toast } from './toast';

// API Base URL - uses Vite proxy in development, direct URL in production
const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

// CSRF token storage - captured from response headers and sent on state-changing requests
let csrfToken: string | null = null;

// API Error Response Type
export interface APIErrorResponse {
  error: string;
  message: string;
  code?: string;
  details?: string;
}

// Types
export interface Session {
  name: string;
  namespace: string;
  user: string;
  template: string;
  state: 'running' | 'hibernated' | 'terminated';
  persistentHome: boolean;
  idleTimeout?: string;
  maxSessionDuration?: string;
  tags?: string[];
  resources?: {
    memory?: string;
    cpu?: string;
  };
  status: SessionStatus;
  createdAt: string;
  activeConnections?: number;
  // Activity tracking fields
  lastActivity?: string;
  idleDuration?: number; // seconds
  idleThreshold?: number; // seconds
  isIdle?: boolean;
  isActive?: boolean;
  // v2.0 multi-platform architecture fields
  agent_id?: string;  // ID of the agent running this session
  platform?: string;  // Platform type (kubernetes, docker, vm, cloud)
  region?: string;    // Region where session is running
}

export interface SessionStatus {
  phase: string;
  podName?: string;
  url?: string;
  lastActivity?: string;
  resourceUsage?: {
    memory?: string;
    cpu?: string;
  };
  conditions?: Array<{
    type: string;
    status: string;
    message: string;
  }>;
}

export interface Template {
  name: string;
  namespace: string;
  displayName: string;
  description: string;
  category: string;
  appType: 'desktop' | 'webapp';
  icon?: string;
  baseImage: string;
  defaultResources?: {
    memory?: string;
    cpu?: string;
  };
  tags?: string[];
  createdAt: string;
}

export interface CatalogTemplate {
  id: number;
  repositoryId: number;
  name: string;
  displayName: string;
  description: string;
  category: string;
  appType: string;
  icon?: string;
  manifest?: string;
  tags: string[];
  installCount: number;
  isFeatured: boolean;
  version: string;
  viewCount: number;
  avgRating: number;
  ratingCount: number;
  createdAt: string;
  updatedAt: string;
  repository: {
    name: string;
    url: string;
  };
}

export interface TemplateRating {
  id: number;
  userId: string;
  username: string;
  fullName: string;
  rating: number;
  review?: string;
  createdAt: string;
  updatedAt: string;
}

export interface CatalogFilters {
  search?: string;
  category?: string;
  tag?: string;
  appType?: string;
  featured?: boolean;
  sort?: 'popular' | 'rating' | 'recent' | 'installs' | 'views';
  page?: number;
  limit?: number;
}

export interface Repository {
  id: number;
  name: string;
  url: string;
  branch: string;
  type: 'template' | 'plugin';
  authType: string;
  lastSync?: string;
  templateCount: number;
  status: string;
  errorMessage?: string;
  createdAt: string;
  updatedAt: string;
}

// Plugin System Types
export interface PluginManifest {
  name: string;
  version: string;
  displayName: string;
  description: string;
  author: string;
  type: string;
  category?: string;
  tags?: string[];
  icon?: string;
  requirements?: PluginRequirements;
  entrypoints?: PluginEntrypoints;
  configSchema?: any;
  defaultConfig?: any;
  permissions?: string[];
  dependencies?: Record<string, string>;
}

export interface PluginRequirements {
  streamspaceVersion?: string;
  dependencies?: Record<string, string>;
}

export interface PluginEntrypoints {
  main?: string;
  ui?: string;
  api?: string;
  webhook?: string;
  cli?: string;
}

export interface CatalogPlugin {
  id: number;
  repositoryId: number;
  name: string;
  version: string;
  displayName: string;
  description: string;
  category: string;
  pluginType: 'extension' | 'webhook' | 'api' | 'ui' | 'theme';
  iconUrl?: string;
  manifest: PluginManifest;
  tags: string[];
  installCount: number;
  avgRating: number;
  ratingCount: number;
  repository: {
    name: string;
    url: string;
  };
  createdAt: string;
  updatedAt: string;
}

export interface InstalledPlugin {
  id: number;
  catalogPluginId?: number;
  name: string;
  version: string;
  enabled: boolean;
  config?: any;
  installedBy: string;
  installedAt: string;
  updatedAt: string;
  displayName?: string;
  description?: string;
  pluginType?: string;
  iconUrl?: string;
  manifest?: PluginManifest;
}

export interface PluginFilters {
  search?: string;
  category?: string;
  pluginType?: 'extension' | 'webhook' | 'api' | 'ui' | 'theme';
  tag?: string;
  sort?: 'popular' | 'rating' | 'recent' | 'installs';
  page?: number;
  limit?: number;
}

export interface PluginRating {
  id: number;
  pluginId: number;
  userId: string;
  username: string;
  fullName: string;
  rating: number;
  review?: string;
  createdAt: string;
  updatedAt: string;
}

// Installed Application Types
export interface InstalledApplication {
  id: string;
  catalogTemplateId: number;
  name: string;
  displayName: string;
  folderPath: string;
  enabled: boolean;
  configuration?: Record<string, any>;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  templateName?: string;
  templateDisplayName?: string;
  description?: string;
  category?: string;
  appType?: string;
  icon?: string;
  manifest?: string;
  groups?: ApplicationGroupAccess[];
}

export interface ApplicationGroupAccess {
  id: string;
  applicationId: string;
  groupId: string;
  accessLevel: 'view' | 'launch' | 'admin';
  createdAt: string;
  groupName?: string;
  groupDisplayName?: string;
}

export interface InstallApplicationRequest {
  catalogTemplateId: number;
  displayName?: string;
  platform?: string;
  configuration?: Record<string, any>;
  groupIds?: string[];
}

export interface UpdateApplicationRequest {
  displayName?: string;
  enabled?: boolean;
  configuration?: Record<string, any>;
}

export interface AddGroupAccessRequest {
  groupId: string;
  accessLevel?: 'view' | 'launch' | 'admin';
}

export interface CreateSessionRequest {
  user: string;
  template?: string;
  applicationId?: string;
  resources?: {
    memory?: string;
    cpu?: string;
  };
  persistentHome?: boolean;
  idleTimeout?: string;
  maxSessionDuration?: string;
  tags?: string[];
}

export interface ConnectSessionResponse {
  connectionId: string;
  sessionUrl: string;
  state: string;
  message: string;
}

// User Management Types
export interface User {
  id: string;
  username: string;
  email: string;
  fullName: string;
  role: 'user' | 'operator' | 'admin';
  provider: 'local' | 'saml' | 'oidc';
  active: boolean;
  createdAt: string;
  updatedAt: string;
  lastLogin?: string;
  quota?: UserQuota;
  groups?: string[];
}

export interface UserQuota {
  userId: string;
  username?: string;  // Add username for admin quota endpoints
  maxSessions: number;
  maxCpu: string;
  maxMemory: string;
  maxStorage: string;
  usedSessions: number;
  usedCpu: string;
  usedMemory: string;
  usedStorage: string;
  // Alternative nested format for compatibility
  limits?: {
    maxSessions: number;
    maxCpu: string;
    maxMemory: string;
    maxStorage: string;
  };
  used?: {
    sessions: number;
    cpu: string;
    memory: string;
    storage: string;
  };
}

export interface CreateUserRequest {
  username: string;
  email: string;
  fullName: string;
  role?: string;
  provider?: string;
  password?: string;
  active?: boolean;
}

export interface UpdateUserRequest {
  fullName?: string;
  email?: string;
  role?: string;
  active?: boolean;
  password?: string;
}

export interface SetQuotaRequest {
  maxSessions?: number;
  maxCpu?: string;
  maxMemory?: string;
  maxStorage?: string;
  username?: string;  // For admin quota endpoints
}

// Group Management Types
export interface Group {
  id: string;
  name: string;
  displayName: string;
  description?: string;
  type: string;
  parentId?: string;
  memberCount?: number;
  quota?: GroupQuota;
  createdAt: string;
  updatedAt: string;
}

export interface GroupQuota {
  groupId: string;
  maxSessions: number;
  maxCpu: string;
  maxMemory: string;
  maxStorage: string;
  usedSessions: number;
  usedCpu: string;
  usedMemory: string;
  usedStorage: string;
}

export interface GroupMember {
  user: User;
  role: string;
  joinedAt: string;
}

export interface CreateGroupRequest {
  name: string;
  displayName: string;
  description?: string;
  type?: string;
  parentId?: string;
}

export interface UpdateGroupRequest {
  displayName?: string;
  description?: string;
  type?: string;
}

export interface AddGroupMemberRequest {
  userId: string;
  role?: string;
}

// Authentication Types
export interface LoginResponse {
  token: string;
  expiresAt: string;
  user: User;
}

export interface RefreshTokenRequest {
  token: string;
}

// ============================================================================
// Integration Hub Types
// ============================================================================

export interface Webhook {
  id: number;
  name: string;
  url: string;
  secret?: string;
  events: string[];
  headers?: Record<string, string>;
  enabled: boolean;
  retry_policy?: {
    max_attempts: number;
    backoff_seconds: number;
  };
  filters?: {
    users?: string[];
    templates?: string[];
    session_states?: string[];
  };
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface WebhookDelivery {
  id: number;
  webhook_id: number;
  event: string;
  payload: any;
  status: 'pending' | 'success' | 'failed';
  attempts: number;
  response_code?: number;
  response_body?: string;
  error_message?: string;
  next_retry_at?: string;
  created_at: string;
}

export interface Integration {
  id: number;
  name: string;
  type: 'slack' | 'teams' | 'discord' | 'pagerduty' | 'email' | 'custom';
  enabled: boolean;
  config: Record<string, any>;
  created_at: string;
}

export interface CreateWebhookRequest {
  name: string;
  url: string;
  secret?: string;
  events: string[];
  enabled?: boolean;
  headers?: Record<string, string>;
}

export interface CreateIntegrationRequest {
  name: string;
  type: string;
  config: Record<string, any>;
}

// ============================================================================
// Security Types
// ============================================================================

export interface MFAMethod {
  id: number;
  user_id: string;
  type: 'totp' | 'sms' | 'email';
  enabled: boolean;
  verified: boolean;
  is_primary: boolean;
  phone_number?: string;
  email?: string;
  created_at: string;
  last_used_at?: string;
}

export interface MFASetupResponse {
  id: number;
  type: string;
  secret?: string;
  qr_code?: string;
  message: string;
}

export interface MFAVerifyRequest {
  code: string;
  method_type?: string;
  trust_device?: boolean;
}

export interface BackupCodesResponse {
  backup_codes: string[];
  message: string;
}

export interface IPWhitelistEntry {
  id: number;
  user_id?: string;
  ip_address: string;
  description?: string;
  enabled: boolean;
  created_by: string;
  created_at: string;
  expires_at?: string;
}

export interface CreateIPWhitelistRequest {
  ip_address: string;
  description?: string;
  user_id?: string;
  expires_at?: string;
}

export interface SecurityAlert {
  type: string;
  severity: 'info' | 'low' | 'medium' | 'high' | 'critical';
  message: string;
  details?: any;
  created_at: string;
}

export interface SessionVerificationResponse {
  verification_id: number;
  risk_score: number;
  risk_level: 'low' | 'medium' | 'high' | 'critical';
  verified: boolean;
  required_action?: string;
  message?: string;
}

// ============================================================================
// Scheduling Types
// ============================================================================

export interface ScheduledSession {
  id: number;
  user_id: string;
  template_id: string;
  name: string;
  description?: string;
  timezone: string;
  schedule: {
    type: 'once' | 'daily' | 'weekly' | 'monthly' | 'cron';
    start_time?: string;
    time_of_day?: string;
    days_of_week?: number[];
    day_of_month?: number;
    cron_expr?: string;
    end_date?: string;
    exceptions?: string[];
  };
  resources?: {
    memory: string;
    cpu: string;
  };
  auto_terminate: boolean;
  terminate_after?: number;
  pre_warm: boolean;
  pre_warm_minutes?: number;
  enabled: boolean;
  next_run_at?: string;
  last_run_at?: string;
  last_session_id?: string;
  last_run_status?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateScheduledSessionRequest {
  template_id: string;
  name: string;
  description?: string;
  timezone: string;
  schedule: ScheduledSession['schedule'];
  resources?: { memory: string; cpu: string };
  auto_terminate?: boolean;
  terminate_after?: number;
  pre_warm?: boolean;
  pre_warm_minutes?: number;
}

export interface CalendarIntegration {
  id: number;
  user_id: string;
  provider: 'google' | 'outlook' | 'ical';
  account_email: string;
  enabled: boolean;
  sync_enabled: boolean;
  auto_create_events: boolean;
  auto_update_events: boolean;
  last_synced_at?: string;
  created_at: string;
}

// ============================================================================
// Load Balancing & Auto-scaling Types
// ============================================================================

export interface LoadBalancingPolicy {
  id: number;
  name: string;
  description?: string;
  strategy: 'round_robin' | 'least_loaded' | 'resource_based' | 'geographic' | 'weighted';
  enabled: boolean;
  session_affinity: boolean;
  health_check_config?: {
    enabled: boolean;
    interval_seconds: number;
    timeout_seconds: number;
    fail_threshold: number;
    pass_threshold: number;
  };
  node_selector?: Record<string, string>;
  node_weights?: Record<string, number>;
  resource_thresholds?: {
    cpu_percent: number;
    memory_percent: number;
    max_sessions: number;
  };
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface NodeStatus {
  node_name: string;
  status: 'ready' | 'not_ready' | 'unknown';
  cpu_allocated: number;
  cpu_capacity: number;
  cpu_percent: number;
  memory_allocated: number;
  memory_capacity: number;
  memory_percent: number;
  active_sessions: number;
  health_status: 'healthy' | 'unhealthy' | 'unknown';
  last_health_check?: string;
  region?: string;
  zone?: string;
  labels?: Record<string, string>;
  weight: number;
}

export interface AutoScalingPolicy {
  id: number;
  name: string;
  description?: string;
  target_type: 'deployment' | 'template';
  target_id: string;
  enabled: boolean;
  scaling_mode: 'horizontal' | 'vertical' | 'both';
  min_replicas: number;
  max_replicas: number;
  metric_type: 'cpu' | 'memory' | 'custom';
  target_metric_value: number;
  scale_up_policy?: {
    threshold: number;
    increment: number;
    stabilization_seconds: number;
  };
  scale_down_policy?: {
    threshold: number;
    increment: number;
    stabilization_seconds: number;
  };
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface ScalingEvent {
  id: number;
  policy_id: number;
  target_type: string;
  target_id: string;
  action: 'scale_up' | 'scale_down';
  previous_replicas: number;
  new_replicas: number;
  trigger: 'manual' | 'metric' | 'schedule';
  metric_value?: number;
  reason?: string;
  status: 'pending' | 'in_progress' | 'completed' | 'failed';
  created_at: string;
}

export interface CreateLoadBalancingPolicyRequest {
  name: string;
  description?: string;
  strategy: string;
  session_affinity?: boolean;
}

export interface CreateAutoScalingPolicyRequest {
  name: string;
  description?: string;
  target_type: string;
  target_id: string;
  scaling_mode: string;
  min_replicas: number;
  max_replicas: number;
  metric_type: string;
  target_metric_value: number;
}

export interface TriggerScalingRequest {
  action: 'scale_up' | 'scale_down';
  replicas?: number;
  reason?: string;
}

// ============================================================================
// Compliance Types
// ============================================================================

export interface ComplianceFramework {
  id: number;
  name: string;
  display_name: string;
  description?: string;
  version?: string;
  enabled: boolean;
  controls?: any[];
  requirements?: Record<string, any>;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CompliancePolicy {
  id: number;
  name: string;
  framework_id: number;
  framework_name?: string;
  applies_to: {
    user_ids?: string[];
    team_ids?: string[];
    roles?: string[];
    all_users: boolean;
  };
  enabled: boolean;
  enforcement_level: 'advisory' | 'warning' | 'blocking';
  data_retention?: {
    session_data_days: number;
    recording_days: number;
    audit_log_days: number;
  };
  access_controls?: {
    require_mfa: boolean;
    allowed_ip_ranges?: string[];
    session_timeout_minutes: number;
  };
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface ComplianceViolation {
  id: number;
  policy_id: number;
  policy_name?: string;
  user_id: string;
  violation_type: string;
  severity: 'low' | 'medium' | 'high' | 'critical';
  description: string;
  details?: any;
  status: 'open' | 'acknowledged' | 'remediated' | 'closed';
  resolution?: string;
  resolved_by?: string;
  resolved_at?: string;
  created_at: string;
}

export interface ComplianceReport {
  id: number;
  framework_id?: number;
  framework_name?: string;
  report_type: 'summary' | 'detailed' | 'attestation';
  report_period: {
    start_date: string;
    end_date: string;
  };
  overall_status: 'compliant' | 'partial' | 'non_compliant';
  controls_summary: {
    total: number;
    compliant: number;
    non_compliant: number;
    unknown: number;
    compliance_rate: number;
  };
  violations?: ComplianceViolation[];
  recommendations?: string[];
  generated_by: string;
  generated_at: string;
}

export interface ComplianceDashboard {
  total_policies: number;
  active_policies: number;
  total_open_violations: number;
  violations_by_severity: {
    critical: number;
    high: number;
    medium: number;
    low: number;
  };
  recent_violations: ComplianceViolation[];
}

export interface CreateCompliancePolicyRequest {
  name: string;
  framework_id: number;
  applies_to: CompliancePolicy['applies_to'];
  enforcement_level: string;
  data_retention?: CompliancePolicy['data_retention'];
  access_controls?: CompliancePolicy['access_controls'];
}

export interface GenerateComplianceReportRequest {
  framework_id?: number;
  report_type: 'summary' | 'detailed' | 'attestation';
  start_date: string;
  end_date: string;
}

class APIClient {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: API_BASE_URL,
      headers: {
        'Content-Type': 'application/json',
      },
      withCredentials: true, // Enable cookies for SAML session
    });

    // Request interceptor for adding auth tokens and CSRF tokens
    this.client.interceptors.request.use(
      (config) => {
        // BUG FIX: Use Zustand persisted store as single source of truth for token
        // Read from 'streamspace-auth' localStorage key (set by Zustand persist middleware)
        const authState = localStorage.getItem('streamspace-auth');
        if (authState) {
          try {
            const parsed = JSON.parse(authState);
            const token = parsed?.state?.token;
            if (token) {
              config.headers.Authorization = `Bearer ${token}`;
            }
          } catch (e) {
            console.error('Failed to parse auth state:', e);
          }
        }

        // Add CSRF token for state-changing requests (POST, PUT, DELETE, PATCH)
        // The server validates this token against the csrf_token cookie
        const method = config.method?.toUpperCase();
        if (csrfToken && (method === 'POST' || method === 'PUT' || method === 'DELETE' || method === 'PATCH')) {
          config.headers['X-CSRF-Token'] = csrfToken;
        }

        return config;
      },
      (error) => Promise.reject(error)
    );

    // Response interceptor for CSRF token capture and error handling
    this.client.interceptors.response.use(
      (response) => {
        // Capture CSRF token from response headers
        // Server sends this on GET/HEAD/OPTIONS requests
        const newCsrfToken = response.headers['x-csrf-token'];
        if (newCsrfToken) {
          csrfToken = newCsrfToken;
        }
        return response;
      },
      (error: AxiosError<APIErrorResponse>) => {
        // Handle network errors
        if (!error.response) {
          toast.error('Network error. Please check your connection.');
          return Promise.reject(error);
        }

        const status = error.response.status;
        const data = error.response.data;

        // Handle different error types
        switch (status) {
          case 401:
            // Unauthorized - clear auth and redirect to login
            if (!window.location.pathname.includes('/login')) {
              toast.error('Session expired. Please log in again.');
              // BUG FIX: Clear Zustand persisted store (single source of truth)
              localStorage.removeItem('streamspace-auth');
              window.location.href = '/login';
            }
            break;

          case 403:
            // Forbidden - quota exceeded or permission denied
            if (data?.code === 'QUOTA_EXCEEDED') {
              toast.error('Resource quota exceeded. ' + (data.message || 'Please delete unused sessions.'));
            } else {
              toast.error(data?.message || 'You do not have permission to perform this action.');
            }
            break;

          case 404:
            // Not found - show friendly message
            toast.error(data?.message || 'Resource not found.');
            break;

          case 409:
            // Conflict - usually duplicate resources
            toast.error(data?.message || 'A conflict occurred. Resource may already exist.');
            break;

          case 429:
            // Rate limit exceeded
            toast.error('Too many requests. Please slow down.');
            break;

          case 500:
          case 502:
          case 503:
          case 504:
            // Server errors
            toast.error(data?.message || 'Server error. Please try again later.');
            break;

          default:
            // Generic error
            if (data?.message) {
              toast.error(data.message);
            } else {
              toast.error('An unexpected error occurred.');
            }
        }

        return Promise.reject(error);
      }
    );
  }

  // ============================================================================
  // Session Management
  // ============================================================================

  async listSessions(user?: string): Promise<Session[]> {
    const params = user ? { user } : {};
    const response = await this.client.get<{ sessions: Session[]; total: number }>('/sessions', { params });
    return response.data.sessions;
  }

  async getSession(id: string): Promise<Session> {
    const response = await this.client.get<Session>(`/sessions/${id}`);
    return response.data;
  }

  async createSession(data: CreateSessionRequest): Promise<Session> {
    const response = await this.client.post<Session>('/sessions', data);
    return response.data;
  }

  async updateSessionState(id: string, state: 'running' | 'hibernated' | 'terminated'): Promise<Session> {
    const response = await this.client.patch<Session>(`/sessions/${id}`, { state });
    return response.data;
  }

  async deleteSession(id: string): Promise<void> {
    await this.client.delete(`/sessions/${id}`);
  }

  async connectSession(id: string, user: string): Promise<ConnectSessionResponse> {
    const response = await this.client.get<ConnectSessionResponse>(`/sessions/${id}/connect`, {
      params: { user },
    });
    return response.data;
  }

  async disconnectSession(id: string, connectionId: string): Promise<void> {
    await this.client.post(`/sessions/${id}/disconnect`, null, {
      params: { connectionId },
    });
  }

  async sendHeartbeat(id: string, connectionId: string): Promise<void> {
    await this.client.post(`/sessions/${id}/heartbeat`, null, {
      params: { connectionId },
    });
  }

  async getSessionConnections(id: string) {
    const response = await this.client.get(`/sessions/${id}/connections`);
    return response.data;
  }

  async updateSessionTags(id: string, tags: string[]): Promise<Session> {
    const response = await this.client.patch<Session>(`/sessions/${id}/tags`, { tags });
    return response.data;
  }

  async listSessionsByTags(tags: string[]): Promise<Session[]> {
    const response = await this.client.get<{ sessions: Session[]; total: number; tags: string[] }>(
      '/sessions/by-tags',
      { params: { tags } }
    );
    return response.data.sessions;
  }

  // ============================================================================
  // Template Management
  // ============================================================================

  async listTemplates(category?: string): Promise<Template[]> {
    const params = category ? { category } : {};
    const response = await this.client.get<{ templates: Template[]; total: number }>('/templates', { params });
    return response.data.templates;
  }

  async getTemplate(id: string): Promise<Template> {
    const response = await this.client.get<Template>(`/templates/${id}`);
    return response.data;
  }

  async createTemplate(data: Partial<Template>): Promise<Template> {
    const response = await this.client.post<Template>('/templates', data);
    return response.data;
  }

  async deleteTemplate(id: string): Promise<void> {
    await this.client.delete(`/templates/${id}`);
  }

  // ============================================================================
  // Catalog (Template Marketplace)
  // ============================================================================

  async listCatalogTemplates(filters?: CatalogFilters): Promise<{ templates: CatalogTemplate[]; total: number; page: number; limit: number; totalPages: number }> {
    const params: Record<string, string> = {};
    if (filters?.search) params.search = filters.search;
    if (filters?.category) params.category = filters.category;
    if (filters?.tag) params.tag = filters.tag;
    if (filters?.appType) params.appType = filters.appType;
    if (filters?.featured) params.featured = 'true';
    if (filters?.sort) params.sort = filters.sort;
    if (filters?.page) params.page = String(filters.page);
    if (filters?.limit) params.limit = String(filters.limit);

    const response = await this.client.get('/catalog/templates', { params });
    return response.data;
  }

  async getTemplateDetails(id: number): Promise<CatalogTemplate> {
    const response = await this.client.get(`/catalog/templates/${id}`);
    return response.data;
  }

  async getFeaturedTemplates(): Promise<CatalogTemplate[]> {
    const response = await this.client.get<{ templates: CatalogTemplate[] }>('/catalog/templates', {
      params: { featured: 'true', limit: '6' }
    });
    return response.data.templates;
  }

  async getTrendingTemplates(): Promise<CatalogTemplate[]> {
    const response = await this.client.get<{ templates: CatalogTemplate[] }>('/catalog/templates', {
      params: { sort: 'recent', limit: '12' }
    });
    return response.data.templates;
  }

  async getPopularTemplates(): Promise<CatalogTemplate[]> {
    const response = await this.client.get<{ templates: CatalogTemplate[] }>('/catalog/templates', {
      params: { sort: 'installs', limit: '12' }
    });
    return response.data.templates;
  }

  // Ratings
  async addTemplateRating(templateId: number, rating: number, review?: string): Promise<void> {
    await this.client.post(`/catalog/templates/${templateId}/ratings`, { rating, review });
  }

  async getTemplateRatings(templateId: number): Promise<{ ratings: TemplateRating[]; total: number }> {
    const response = await this.client.get(`/catalog/templates/${templateId}/ratings`);
    return response.data;
  }

  async updateTemplateRating(templateId: number, ratingId: number, rating: number, review?: string): Promise<void> {
    await this.client.put(`/catalog/templates/${templateId}/ratings/${ratingId}`, { rating, review });
  }

  async deleteTemplateRating(templateId: number, ratingId: number): Promise<void> {
    await this.client.delete(`/catalog/templates/${templateId}/ratings/${ratingId}`);
  }

  // Analytics
  async recordTemplateView(templateId: number): Promise<void> {
    await this.client.post(`/catalog/templates/${templateId}/view`);
  }

  async recordTemplateInstall(templateId: number): Promise<void> {
    await this.client.post(`/catalog/templates/${templateId}/install`);
  }

  async installCatalogTemplate(id: number): Promise<void> {
    await this.client.post(`/catalog/templates/${id}/install`);
  }

  // ============================================================================
  // Installed Applications Management
  // ============================================================================

  async listApplications(enabledOnly?: boolean): Promise<{ applications: InstalledApplication[]; total: number }> {
    const params: Record<string, string> = { _t: Date.now().toString() }; // Cache bust
    if (enabledOnly) params.enabled = 'true';
    const response = await this.client.get<{ applications: InstalledApplication[]; total: number }>('/applications', { params });
    return response.data;
  }

  async installApplication(request: InstallApplicationRequest): Promise<InstalledApplication> {
    const response = await this.client.post<InstalledApplication>('/applications', request);
    return response.data;
  }

  async getApplication(id: string): Promise<InstalledApplication> {
    const response = await this.client.get<InstalledApplication>(`/applications/${id}`);
    return response.data;
  }

  async updateApplication(id: string, request: UpdateApplicationRequest): Promise<InstalledApplication> {
    const response = await this.client.put<InstalledApplication>(`/applications/${id}`, request);
    return response.data;
  }

  async deleteApplication(id: string): Promise<void> {
    await this.client.delete(`/applications/${id}`);
  }

  async setApplicationEnabled(id: string, enabled: boolean): Promise<void> {
    await this.client.put(`/applications/${id}/enabled`, { enabled });
  }

  async getApplicationGroups(id: string): Promise<{ groups: ApplicationGroupAccess[]; total: number }> {
    const response = await this.client.get<{ groups: ApplicationGroupAccess[]; total: number }>(`/applications/${id}/groups`);
    return response.data;
  }

  async addApplicationGroupAccess(id: string, request: AddGroupAccessRequest): Promise<void> {
    await this.client.post(`/applications/${id}/groups`, request);
  }

  async updateApplicationGroupAccess(id: string, groupId: string, accessLevel: string): Promise<void> {
    await this.client.put(`/applications/${id}/groups/${groupId}`, { accessLevel });
  }

  async removeApplicationGroupAccess(id: string, groupId: string): Promise<void> {
    await this.client.delete(`/applications/${id}/groups/${groupId}`);
  }

  async getApplicationTemplateConfig(id: string): Promise<{ config: any }> {
    const response = await this.client.get<{ config: any }>(`/applications/${id}/config`);
    return response.data;
  }

  async getUserApplications(): Promise<{ applications: InstalledApplication[]; total: number }> {
    const response = await this.client.get<{ applications: InstalledApplication[]; total: number }>('/applications/user');
    return response.data;
  }

  // ============================================================================
  // Repository Management
  // ============================================================================

  async listRepositories(): Promise<Repository[]> {
    const response = await this.client.get<{ repositories: Repository[]; total: number }>('/catalog/repositories');
    return response.data.repositories;
  }

  async addRepository(data: {
    name: string;
    url: string;
    branch?: string;
    authType?: string;
    authSecret?: string;
  }): Promise<{ id: number; message: string }> {
    const response = await this.client.post('/catalog/repositories', data);
    return response.data;
  }

  async syncRepository(id: number): Promise<void> {
    await this.client.post(`/catalog/repositories/${id}/sync`);
  }

  async syncAllRepositories(): Promise<void> {
    await this.client.post('/catalog/sync');
  }

  async updateRepository(id: number, data: {
    name?: string;
    url?: string;
    branch?: string;
    authType?: string;
    authSecret?: string;
  }): Promise<void> {
    await this.client.put(`/catalog/repositories/${id}`, data);
  }

  async deleteRepository(id: number): Promise<void> {
    await this.client.delete(`/catalog/repositories/${id}`);
  }

  async getRepositoryStats(id: number): Promise<{
    templateCount: number;
    syncHistory: Array<{
      timestamp: string;
      status: string;
      duration?: number;
      error?: string;
    }>;
  }> {
    const response = await this.client.get(`/catalog/repositories/${id}/stats`);
    return response.data;
  }

  // ============================================================================
  // Plugin Management
  // ============================================================================

  // Plugin Catalog
  async browsePlugins(filters?: PluginFilters): Promise<{ plugins: CatalogPlugin[]; total: number }> {
    const params: Record<string, any> = {};
    if (filters?.search) params.search = filters.search;
    if (filters?.category) params.category = filters.category;
    if (filters?.pluginType) params.type = filters.pluginType;
    if (filters?.tag) params.tag = filters.tag;
    if (filters?.sort) params.sort = filters.sort;
    if (filters?.page) params.page = filters.page;
    if (filters?.limit) params.limit = filters.limit;

    const response = await this.client.get<{ plugins: CatalogPlugin[]; total: number }>('/plugins/catalog', { params });
    return response.data;
  }

  async getPluginDetails(id: number): Promise<CatalogPlugin> {
    const response = await this.client.get<CatalogPlugin>(`/plugins/catalog/${id}`);
    return response.data;
  }

  async ratePlugin(id: number, rating: number, review?: string): Promise<void> {
    await this.client.post(`/plugins/catalog/${id}/rate`, { rating, review });
  }

  async getPluginRatings(id: number): Promise<PluginRating[]> {
    const response = await this.client.get<{ ratings: PluginRating[] }>(`/plugins/catalog/${id}/ratings`);
    return response.data.ratings;
  }

  async installPlugin(id: number, config?: any): Promise<{ pluginId: number; message: string }> {
    const response = await this.client.post(`/plugins/catalog/${id}/install`, { config });
    return response.data;
  }

  // Installed Plugins
  async listInstalledPlugins(enabledOnly?: boolean): Promise<InstalledPlugin[]> {
    const params = enabledOnly ? { enabled: 'true' } : {};
    const response = await this.client.get<{ plugins: InstalledPlugin[] }>('/plugins', { params });
    // BUG FIX P0-123: Guard against null/undefined plugins response
    return Array.isArray(response.data.plugins) ? response.data.plugins : [];
  }

  async getInstalledPlugin(id: number): Promise<InstalledPlugin> {
    const response = await this.client.get<InstalledPlugin>(`/plugins/${id}`);
    return response.data;
  }

  async updatePluginConfig(id: number, config: any): Promise<void> {
    await this.client.patch(`/plugins/${id}`, { config });
  }

  async uninstallPlugin(id: number): Promise<void> {
    await this.client.delete(`/plugins/${id}`);
  }

  async enablePlugin(id: number): Promise<void> {
    await this.client.post(`/plugins/${id}/enable`);
  }

  async disablePlugin(id: number): Promise<void> {
    await this.client.post(`/plugins/${id}/disable`);
  }

  // ============================================================================
  // Session Sharing & Collaboration
  // ============================================================================

  async createShare(sessionId: string, data: {
    sharedWithUserId: string;
    permissionLevel: 'view' | 'collaborate' | 'control';
    expiresAt?: string;
  }): Promise<{ id: string; shareToken: string; message: string }> {
    const response = await this.client.post(`/sessions/${sessionId}/share`, data);
    return response.data;
  }

  async listShares(sessionId: string): Promise<Array<{
    id: string;
    sessionId: string;
    ownerUserId: string;
    sharedWithUserId: string;
    permissionLevel: string;
    shareToken: string;
    createdAt: string;
    expiresAt?: string;
    acceptedAt?: string;
    user: {
      id: string;
      username: string;
      fullName: string;
      email: string;
    };
  }>> {
    const response = await this.client.get(`/sessions/${sessionId}/shares`);
    return response.data.shares;
  }

  async revokeShare(sessionId: string, shareId: string): Promise<void> {
    await this.client.delete(`/sessions/${sessionId}/shares/${shareId}`);
  }

  async transferOwnership(sessionId: string, newOwnerUserId: string): Promise<void> {
    await this.client.post(`/sessions/${sessionId}/transfer`, { newOwnerUserId });
  }

  async createInvitation(sessionId: string, data: {
    permissionLevel: 'view' | 'collaborate' | 'control';
    maxUses?: number;
    expiresAt?: string;
  }): Promise<{ id: string; invitationToken: string; message: string }> {
    const response = await this.client.post(`/sessions/${sessionId}/invitations`, data);
    return response.data;
  }

  async listInvitations(sessionId: string): Promise<Array<{
    id: string;
    sessionId: string;
    createdBy: string;
    invitationToken: string;
    permissionLevel: string;
    maxUses: number;
    useCount: number;
    expiresAt?: string;
    createdAt: string;
    isExpired?: boolean;
    isExhausted?: boolean;
  }>> {
    const response = await this.client.get(`/sessions/${sessionId}/invitations`);
    return response.data.invitations;
  }

  async revokeInvitation(token: string): Promise<void> {
    await this.client.delete(`/invitations/${token}`);
  }

  async acceptInvitation(token: string, userId: string): Promise<{ sessionId: string; message: string }> {
    const response = await this.client.post(`/invitations/${token}/accept`, { userId });
    return response.data;
  }

  async listCollaborators(sessionId: string): Promise<Array<{
    id: string;
    sessionId: string;
    userId: string;
    permissionLevel: string;
    joinedAt: string;
    lastActivity: string;
    isActive: boolean;
    user: {
      username: string;
      fullName: string;
    };
  }>> {
    const response = await this.client.get(`/sessions/${sessionId}/collaborators`);
    return response.data.collaborators;
  }

  async updateCollaboratorActivity(sessionId: string, userId: string): Promise<void> {
    await this.client.post(`/sessions/${sessionId}/collaborators/${userId}/activity`);
  }

  async removeCollaborator(sessionId: string, userId: string): Promise<void> {
    await this.client.delete(`/sessions/${sessionId}/collaborators/${userId}`);
  }

  async listSharedSessions(userId: string): Promise<Array<{
    id: string;
    ownerUserId: string;
    ownerUsername: string;
    templateName: string;
    state: string;
    appType: string;
    createdAt: string;
    sharedAt: string;
    permissionLevel: string;
    isShared: boolean;
    url?: string;
  }>> {
    const response = await this.client.get(`/shared-sessions?userId=${userId}`);
    return response.data.sessions;
  }

  // ============================================================================
  // Authentication
  // ============================================================================

  async login(username: string, password: string): Promise<LoginResponse> {
    const response = await this.client.post<LoginResponse>('/auth/login', { username, password });
    return response.data;
  }

  async refreshToken(token: string): Promise<LoginResponse> {
    const response = await this.client.post<LoginResponse>('/auth/refresh', { token });
    return response.data;
  }

  async logout(): Promise<void> {
    await this.client.post('/auth/logout');
    // BUG FIX: Clear Zustand persisted store (single source of truth)
    localStorage.removeItem('streamspace-auth');
  }

  async samlLogin(): Promise<{ redirectUrl: string }> {
    const response = await this.client.get('/auth/saml/login');
    return response.data;
  }

  async changePassword(oldPassword: string, newPassword: string): Promise<void> {
    await this.client.post('/auth/change-password', { oldPassword, newPassword });
  }

  // Setup Wizard (First-Run Admin Onboarding)
  async getSetupStatus(): Promise<{ setupRequired: boolean; adminExists: boolean; hasPassword: boolean; message?: string }> {
    const response = await this.client.get('/auth/setup/status');
    return response.data;
  }

  async setupAdmin(password: string, passwordConfirm: string, email: string): Promise<{ message: string; username: string; email: string }> {
    const response = await this.client.post('/auth/setup', { password, passwordConfirm, email });
    return response.data;
  }

  // ============================================================================
  // User Management
  // ============================================================================

  async listUsers(role?: string, provider?: string, active?: boolean): Promise<User[]> {
    const params: Record<string, string> = {};
    if (role) params.role = role;
    if (provider) params.provider = provider;
    if (active !== undefined) params.active = String(active);

    const response = await this.client.get<{ users: User[]; total: number }>('/users', { params });
    return response.data.users;
  }

  async getUser(id: string): Promise<User> {
    const response = await this.client.get<User>(`/users/${id}`);
    return response.data;
  }

  async getCurrentUser(): Promise<User> {
    const response = await this.client.get<User>('/users/me');
    return response.data;
  }

  async createUser(data: CreateUserRequest): Promise<User> {
    const response = await this.client.post<User>('/users', data);
    return response.data;
  }

  async updateUser(id: string, data: UpdateUserRequest): Promise<User> {
    const response = await this.client.patch<User>(`/users/${id}`, data);
    return response.data;
  }

  async deleteUser(id: string): Promise<void> {
    await this.client.delete(`/users/${id}`);
  }

  async getCurrentUserQuota(): Promise<UserQuota> {
    const response = await this.client.get<UserQuota>('/users/me/quota');
    return response.data;
  }

  async getUserQuota(id: string): Promise<UserQuota> {
    const response = await this.client.get<UserQuota>(`/users/${id}/quota`);
    return response.data;
  }

  async setUserQuota(id: string, data: SetQuotaRequest): Promise<UserQuota> {
    const response = await this.client.put<UserQuota>(`/users/${id}/quota`, data);
    return response.data;
  }

  async getUserGroups(id: string): Promise<{ groups: Group[]; total: number }> {
    const response = await this.client.get<{ groups: Group[]; total: number }>(`/users/${id}/groups`);
    return response.data;
  }

  // ============================================================================
  // Group Management
  // ============================================================================

  async listGroups(type?: string, parentId?: string): Promise<Group[]> {
    const params: Record<string, string> = {};
    if (type) params.type = type;
    if (parentId) params.parentId = parentId;

    const response = await this.client.get<{ groups: Group[]; total: number }>('/groups', { params });
    return response.data.groups;
  }

  async getGroup(id: string): Promise<Group> {
    const response = await this.client.get<Group>(`/groups/${id}`);
    return response.data;
  }

  async createGroup(data: CreateGroupRequest): Promise<Group> {
    const response = await this.client.post<Group>('/groups', data);
    return response.data;
  }

  async updateGroup(id: string, data: UpdateGroupRequest): Promise<Group> {
    const response = await this.client.patch<Group>(`/groups/${id}`, data);
    return response.data;
  }

  async deleteGroup(id: string): Promise<void> {
    await this.client.delete(`/groups/${id}`);
  }

  async getGroupMembers(id: string): Promise<{ members: GroupMember[]; total: number }> {
    const response = await this.client.get<{ members: GroupMember[]; total: number }>(`/groups/${id}/members`);
    return response.data;
  }

  async addGroupMember(id: string, data: AddGroupMemberRequest): Promise<void> {
    await this.client.post(`/groups/${id}/members`, data);
  }

  async removeGroupMember(id: string, userId: string): Promise<void> {
    await this.client.delete(`/groups/${id}/members/${userId}`);
  }

  async updateMemberRole(id: string, userId: string, role: string): Promise<void> {
    await this.client.patch(`/groups/${id}/members/${userId}`, { role });
  }

  async getGroupQuota(id: string): Promise<GroupQuota> {
    const response = await this.client.get<GroupQuota>(`/groups/${id}/quota`);
    return response.data;
  }

  async setGroupQuota(id: string, data: SetQuotaRequest): Promise<GroupQuota> {
    const response = await this.client.put<GroupQuota>(`/groups/${id}/quota`, data);
    return response.data;
  }

  // ============================================================================
  // Node Management (Admin)
  // ============================================================================

  async listNodes(): Promise<any[]> {
    const response = await this.client.get('/admin/nodes');
    return response.data;
  }

  async getNode(name: string): Promise<any> {
    const response = await this.client.get(`/admin/nodes/${name}`);
    return response.data;
  }

  async getClusterStats(): Promise<any> {
    const response = await this.client.get('/admin/nodes/stats');
    return response.data;
  }

  async addNodeLabel(name: string, key: string, value: string): Promise<void> {
    await this.client.put(`/admin/nodes/${name}/labels`, { key, value });
  }

  async removeNodeLabel(name: string, key: string): Promise<void> {
    await this.client.delete(`/admin/nodes/${name}/labels/${key}`);
  }

  async addNodeTaint(name: string, taint: { key: string; value: string; effect: string }): Promise<void> {
    await this.client.post(`/admin/nodes/${name}/taints`, taint);
  }

  async removeNodeTaint(name: string, key: string): Promise<void> {
    await this.client.delete(`/admin/nodes/${name}/taints/${key}`);
  }

  async cordonNode(name: string): Promise<void> {
    await this.client.post(`/admin/nodes/${name}/cordon`);
  }

  async uncordonNode(name: string): Promise<void> {
    await this.client.post(`/admin/nodes/${name}/uncordon`);
  }

  async drainNode(name: string, gracePeriodSeconds?: number): Promise<void> {
    await this.client.post(`/admin/nodes/${name}/drain`, { grace_period_seconds: gracePeriodSeconds });
  }

  // ============================================================================
  // User Quota Management (Admin)
  // ============================================================================

  async listAllUserQuotas(): Promise<UserQuota[]> {
    const response = await this.client.get<{ quotas: UserQuota[] }>('/admin/quotas');
    return response.data.quotas;
  }

  async getAdminUserQuota(username: string): Promise<UserQuota> {
    const response = await this.client.get<UserQuota>(`/admin/quotas/${username}`);
    return response.data;
  }

  async setAdminUserQuota(data: SetQuotaRequest): Promise<UserQuota> {
    const response = await this.client.put<UserQuota>('/admin/quotas', data);
    return response.data;
  }

  async deleteAdminUserQuota(username: string): Promise<void> {
    await this.client.delete(`/admin/quotas/${username}`);
  }

  // ============================================================================
  // Health & Metrics
  // ============================================================================

  async getHealth() {
    const response = await this.client.get('/health');
    return response.data;
  }

  async getVersion() {
    const response = await this.client.get('/version');
    return response.data;
  }

  async getMetrics() {
    const response = await this.client.get('/metrics');
    return response.data;
  }

  // ============================================================================
  // Integration Hub
  // ============================================================================

  async listWebhooks(): Promise<{ webhooks: Webhook[] }> {
    const response = await this.client.get<{ webhooks: Webhook[] }>('/integrations/webhooks');
    return response.data;
  }

  async createWebhook(data: CreateWebhookRequest): Promise<Webhook> {
    const response = await this.client.post<Webhook>('/integrations/webhooks', data);
    return response.data;
  }

  async updateWebhook(id: number, data: Partial<CreateWebhookRequest>): Promise<Webhook> {
    const response = await this.client.patch<Webhook>(`/integrations/webhooks/${id}`, data);
    return response.data;
  }

  async deleteWebhook(id: number): Promise<void> {
    await this.client.delete(`/integrations/webhooks/${id}`);
  }

  async testWebhook(id: number): Promise<{ message: string; delivery_id: number }> {
    const response = await this.client.post(`/integrations/webhooks/${id}/test`);
    return response.data;
  }

  async getWebhookDeliveries(webhookId: number): Promise<{ deliveries: WebhookDelivery[] }> {
    const response = await this.client.get<{ deliveries: WebhookDelivery[] }>(
      `/integrations/webhooks/${webhookId}/deliveries`
    );
    return response.data;
  }

  async retryWebhookDelivery(webhookId: number, deliveryId: number): Promise<void> {
    await this.client.post(`/integrations/webhooks/${webhookId}/retry/${deliveryId}`);
  }

  async listIntegrations(): Promise<{ integrations: Integration[] }> {
    const response = await this.client.get<{ integrations: Integration[] }>('/integrations/external');
    return response.data;
  }

  async createIntegration(data: CreateIntegrationRequest): Promise<Integration> {
    const response = await this.client.post<Integration>('/integrations/external', data);
    return response.data;
  }

  async deleteIntegration(id: number): Promise<void> {
    await this.client.delete(`/integrations/external/${id}`);
  }

  async testIntegration(id: number): Promise<{ message: string }> {
    const response = await this.client.post(`/integrations/external/${id}/test`);
    return response.data;
  }

  async getAvailableEvents(): Promise<{ events: string[] }> {
    const response = await this.client.get<{ events: string[] }>('/integrations/events');
    return response.data;
  }

  // ============================================================================
  // Security
  // ============================================================================

  async setupMFA(type: 'totp' | 'sms' | 'email', data?: { phone_number?: string; email?: string }): Promise<MFASetupResponse> {
    const response = await this.client.post<MFASetupResponse>('/security/mfa/setup', { type, ...data });
    return response.data;
  }

  async verifyMFASetup(mfaId: number, code: string): Promise<BackupCodesResponse> {
    const response = await this.client.post<BackupCodesResponse>(`/security/mfa/${mfaId}/verify-setup`, { code });
    return response.data;
  }

  async verifyMFA(data: MFAVerifyRequest): Promise<{ message: string; verified: boolean }> {
    const response = await this.client.post('/security/mfa/verify', data);
    return response.data;
  }

  async listMFAMethods(): Promise<{ methods: MFAMethod[] }> {
    const response = await this.client.get<{ methods: MFAMethod[] }>('/security/mfa/methods');
    return response.data;
  }

  async disableMFA(mfaId: number): Promise<{ message: string }> {
    const response = await this.client.delete(`/security/mfa/${mfaId}`);
    return response.data;
  }

  async generateBackupCodes(): Promise<BackupCodesResponse> {
    const response = await this.client.post<BackupCodesResponse>('/security/mfa/backup-codes');
    return response.data;
  }

  async createIPWhitelist(data: CreateIPWhitelistRequest): Promise<{ id: number; message: string }> {
    const response = await this.client.post('/security/ip-whitelist', data);
    return response.data;
  }

  async listIPWhitelist(userId?: string): Promise<{ entries: IPWhitelistEntry[] }> {
    const params = userId ? { user_id: userId } : {};
    const response = await this.client.get<{ entries: IPWhitelistEntry[] }>('/security/ip-whitelist', { params });
    return response.data;
  }

  async deleteIPWhitelist(entryId: number): Promise<{ message: string }> {
    const response = await this.client.delete(`/security/ip-whitelist/${entryId}`);
    return response.data;
  }

  async checkIPAccess(ipAddress?: string, userId?: string): Promise<{ allowed: boolean; ip_address: string }> {
    const params: any = {};
    if (ipAddress) params.ip_address = ipAddress;
    if (userId) params.user_id = userId;
    const response = await this.client.get('/security/ip-whitelist/check', { params });
    return response.data;
  }

  async verifySession(sessionId: string): Promise<SessionVerificationResponse> {
    const response = await this.client.post<SessionVerificationResponse>(`/security/sessions/${sessionId}/verify`);
    return response.data;
  }

  async checkDevicePosture(data: any): Promise<{ compliant: boolean; issues: string[] }> {
    const response = await this.client.post('/security/device-posture', data);
    return response.data;
  }

  async getSecurityAlerts(): Promise<{ alerts: SecurityAlert[] }> {
    const response = await this.client.get<{ alerts: SecurityAlert[] }>('/security/alerts');
    return response.data;
  }

  // ============================================================================
  // Scheduling
  // ============================================================================

  async listScheduledSessions(): Promise<{ schedules: ScheduledSession[]; count: number }> {
    const response = await this.client.get<{ schedules: ScheduledSession[]; count: number }>('/scheduling/sessions');
    return response.data;
  }

  async getScheduledSession(id: number): Promise<ScheduledSession> {
    const response = await this.client.get<ScheduledSession>(`/scheduling/sessions/${id}`);
    return response.data;
  }

  async createScheduledSession(data: CreateScheduledSessionRequest): Promise<{ id: number; message: string; schedule: ScheduledSession }> {
    const response = await this.client.post('/scheduling/sessions', data);
    return response.data;
  }

  async updateScheduledSession(id: number, data: Partial<CreateScheduledSessionRequest>): Promise<{ message: string }> {
    const response = await this.client.patch(`/scheduling/sessions/${id}`, data);
    return response.data;
  }

  async deleteScheduledSession(id: number): Promise<{ message: string }> {
    const response = await this.client.delete(`/scheduling/sessions/${id}`);
    return response.data;
  }

  async enableScheduledSession(id: number): Promise<{ message: string }> {
    const response = await this.client.post(`/scheduling/sessions/${id}/enable`);
    return response.data;
  }

  async disableScheduledSession(id: number): Promise<{ message: string }> {
    const response = await this.client.post(`/scheduling/sessions/${id}/disable`);
    return response.data;
  }

  async connectCalendar(provider: 'google' | 'outlook'): Promise<{ provider: string; auth_url: string; message: string }> {
    const response = await this.client.post('/scheduling/calendar/connect', { provider });
    return response.data;
  }

  async listCalendarIntegrations(): Promise<{ integrations: CalendarIntegration[] }> {
    const response = await this.client.get<{ integrations: CalendarIntegration[] }>('/scheduling/calendar/integrations');
    return response.data;
  }

  async disconnectCalendar(integrationId: number): Promise<{ message: string }> {
    const response = await this.client.delete(`/scheduling/calendar/integrations/${integrationId}`);
    return response.data;
  }

  async syncCalendar(integrationId: number): Promise<{ message: string; synced_at: string }> {
    const response = await this.client.post(`/scheduling/calendar/integrations/${integrationId}/sync`);
    return response.data;
  }

  async exportICalendar(): Promise<Blob> {
    const response = await this.client.get('/scheduling/calendar/export.ics', {
      responseType: 'blob',
    });
    return response.data;
  }

  // ============================================================================
  // Load Balancing & Auto-scaling
  // ============================================================================

  async listLoadBalancingPolicies(): Promise<{ policies: LoadBalancingPolicy[] }> {
    const response = await this.client.get<{ policies: LoadBalancingPolicy[] }>('/scaling/load-balancing/policies');
    return response.data;
  }

  async createLoadBalancingPolicy(data: CreateLoadBalancingPolicyRequest): Promise<{ id: number; policy: LoadBalancingPolicy }> {
    const response = await this.client.post('/scaling/load-balancing/policies', data);
    return response.data;
  }

  async getNodeStatus(): Promise<{ nodes: NodeStatus[]; cluster_summary: any }> {
    const response = await this.client.get<{ nodes: NodeStatus[]; cluster_summary: any }>('/scaling/load-balancing/nodes');
    return response.data;
  }

  async selectNode(data: {
    policy_id?: number;
    required_cpu: number;
    required_memory: number;
    user_location?: string;
  }): Promise<{ node_name: string; strategy_used: string; cpu_available: number; memory_available: number }> {
    const response = await this.client.post('/scaling/load-balancing/select-node', data);
    return response.data;
  }

  async listAutoScalingPolicies(): Promise<{ policies: AutoScalingPolicy[] }> {
    const response = await this.client.get<{ policies: AutoScalingPolicy[] }>('/scaling/autoscaling/policies');
    return response.data;
  }

  async createAutoScalingPolicy(data: CreateAutoScalingPolicyRequest): Promise<{ id: number; policy: AutoScalingPolicy }> {
    const response = await this.client.post('/scaling/autoscaling/policies', data);
    return response.data;
  }

  async triggerScaling(policyId: number, data: TriggerScalingRequest): Promise<{ event_id: number; action: string; previous_replicas: number; new_replicas: number }> {
    const response = await this.client.post(`/scaling/autoscaling/policies/${policyId}/trigger`, data);
    return response.data;
  }

  async getScalingHistory(policyId?: number, limit: number = 50): Promise<{ events: ScalingEvent[]; count: number }> {
    const params: any = { limit };
    if (policyId) params.policy_id = policyId;
    const response = await this.client.get<{ events: ScalingEvent[]; count: number }>('/scaling/autoscaling/history', { params });
    return response.data;
  }

  // ============================================================================
  // Compliance
  // ============================================================================

  async listComplianceFrameworks(): Promise<{ frameworks: ComplianceFramework[] }> {
    const response = await this.client.get<{ frameworks: ComplianceFramework[] }>('/compliance/frameworks');
    return response.data;
  }

  async createComplianceFramework(data: {
    name: string;
    display_name: string;
    description?: string;
    version?: string;
  }): Promise<{ id: number; framework: ComplianceFramework }> {
    const response = await this.client.post('/compliance/frameworks', data);
    return response.data;
  }

  async listCompliancePolicies(): Promise<{ policies: CompliancePolicy[] }> {
    const response = await this.client.get<{ policies: CompliancePolicy[] }>('/compliance/policies');
    return response.data;
  }

  async createCompliancePolicy(data: CreateCompliancePolicyRequest): Promise<{ id: number; policy: CompliancePolicy }> {
    const response = await this.client.post('/compliance/policies', data);
    return response.data;
  }

  async listComplianceViolations(params?: {
    user_id?: string;
    policy_id?: string;
    status?: string;
    severity?: string;
  }): Promise<{ violations: ComplianceViolation[] }> {
    const response = await this.client.get<{ violations: ComplianceViolation[] }>('/compliance/violations', { params });
    return response.data;
  }

  async recordComplianceViolation(data: {
    policy_id: number;
    user_id: string;
    violation_type: string;
    severity: string;
    description: string;
    details?: any;
  }): Promise<{ id: number; violation: ComplianceViolation }> {
    const response = await this.client.post('/compliance/violations', data);
    return response.data;
  }

  async resolveComplianceViolation(violationId: number, data: {
    resolution: string;
    status: 'acknowledged' | 'remediated' | 'closed';
  }): Promise<{ message: string }> {
    const response = await this.client.post(`/compliance/violations/${violationId}/resolve`, data);
    return response.data;
  }

  async generateComplianceReport(data: GenerateComplianceReportRequest): Promise<ComplianceReport> {
    const response = await this.client.post<ComplianceReport>('/compliance/reports/generate', data);
    return response.data;
  }

  async getComplianceDashboard(): Promise<ComplianceDashboard> {
    const response = await this.client.get<ComplianceDashboard>('/compliance/dashboard');
    return response.data;
  }

  // ============================================================================
  // User Preferences & Favorites
  // ============================================================================

  async getFavorites(): Promise<{ favorites: { templateName: string; addedAt: string }[]; total: number }> {
    const response = await this.client.get('/preferences/favorites');
    return response.data;
  }

  async addFavorite(templateName: string): Promise<{ message: string; templateName: string }> {
    const response = await this.client.post(`/preferences/favorites/${encodeURIComponent(templateName)}`);
    return response.data;
  }

  async removeFavorite(templateName: string): Promise<{ message: string; templateName: string }> {
    const response = await this.client.delete(`/preferences/favorites/${encodeURIComponent(templateName)}`);
    return response.data;
  }

  // ============================================================================
  // Agent Management (Admin)
  // ============================================================================

  async listAgents(params?: {
    platform?: string;
    status?: string;
    approval_status?: string;
    page?: number;
    limit?: number;
  }): Promise<{ agents: any[]; total: number; page: number; limit: number }> {
    const queryParams = new URLSearchParams();
    if (params?.platform) queryParams.append('platform', params.platform);
    if (params?.status) queryParams.append('status', params.status);
    if (params?.approval_status) queryParams.append('approval_status', params.approval_status);
    if (params?.page) queryParams.append('page', String(params.page));
    if (params?.limit) queryParams.append('limit', String(params.limit));

    const response = await this.client.get(`/admin/agents?${queryParams.toString()}`);
    return response.data;
  }

  async deleteAgent(agentId: string): Promise<void> {
    await this.client.delete(`/admin/agents/${agentId}`);
  }

  async approveAgent(agentId: string): Promise<void> {
    await this.client.post(`/admin/agents/${agentId}/approve`);
  }

  async rejectAgent(agentId: string): Promise<void> {
    await this.client.post(`/admin/agents/${agentId}/reject`);
  }
}

// Export singleton instance
export const api = new APIClient();
export default api;
