import axios, { AxiosInstance } from 'axios';

// API Base URL - uses Vite proxy in development, direct URL in production
const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

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
  resources?: {
    memory?: string;
    cpu?: string;
  };
  status: SessionStatus;
  createdAt: string;
  activeConnections?: number;
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
  name: string;
  displayName: string;
  description: string;
  category: string;
  icon?: string;
  manifest: string;
  tags: string[];
  installCount: number;
  repository: {
    name: string;
    url: string;
  };
}

export interface Repository {
  id: number;
  name: string;
  url: string;
  branch: string;
  authType: string;
  lastSync?: string;
  templateCount: number;
  status: string;
  errorMessage?: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateSessionRequest {
  user: string;
  template: string;
  resources?: {
    memory?: string;
    cpu?: string;
  };
  persistentHome?: boolean;
  idleTimeout?: string;
  maxSessionDuration?: string;
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
  maxSessions: number;
  maxCpu: string;
  maxMemory: string;
  maxStorage: string;
  usedSessions: number;
  usedCpu: string;
  usedMemory: string;
  usedStorage: string;
}

export interface CreateUserRequest {
  username: string;
  email: string;
  fullName: string;
  role?: string;
  provider?: string;
  password?: string;
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

    // Request interceptor for adding auth tokens
    this.client.interceptors.request.use(
      (config) => {
        // Get JWT token from localStorage
        const token = localStorage.getItem('streamspace_token');
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;
        }
        return config;
      },
      (error) => Promise.reject(error)
    );

    // Response interceptor for error handling
    this.client.interceptors.response.use(
      (response) => response,
      (error) => {
        if (error.response?.status === 401) {
          // Clear token and redirect to login
          localStorage.removeItem('streamspace_token');
          localStorage.removeItem('streamspace_user');
          window.location.href = '/login';
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

  async listCatalogTemplates(category?: string, tag?: string): Promise<CatalogTemplate[]> {
    const params: Record<string, string> = {};
    if (category) params.category = category;
    if (tag) params.tag = tag;

    const response = await this.client.get<{ templates: CatalogTemplate[]; total: number }>('/catalog/templates', {
      params,
    });
    return response.data.templates;
  }

  async installCatalogTemplate(id: number): Promise<void> {
    await this.client.post(`/catalog/templates/${id}/install`);
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

  async deleteRepository(id: number): Promise<void> {
    await this.client.delete(`/catalog/repositories/${id}`);
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
    localStorage.removeItem('streamspace_token');
    localStorage.removeItem('streamspace_user');
  }

  async samlLogin(): Promise<{ redirectUrl: string }> {
    const response = await this.client.get('/auth/saml/login');
    return response.data;
  }

  async changePassword(oldPassword: string, newPassword: string): Promise<void> {
    await this.client.post('/auth/change-password', { oldPassword, newPassword });
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

  async listUserQuotas(): Promise<UserQuota[]> {
    const response = await this.client.get<{ quotas: UserQuota[] }>('/admin/quotas');
    return response.data.quotas;
  }

  async getUserQuota(username: string): Promise<UserQuota> {
    const response = await this.client.get<UserQuota>(`/admin/quotas/${username}`);
    return response.data;
  }

  async setUserQuota(data: SetQuotaRequest): Promise<UserQuota> {
    const response = await this.client.put<UserQuota>('/admin/quotas', data);
    return response.data;
  }

  async deleteUserQuota(username: string): Promise<void> {
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
}

// Export singleton instance
export const api = new APIClient();
export default api;
