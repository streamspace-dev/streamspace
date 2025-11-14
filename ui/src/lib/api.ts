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
}

// Export singleton instance
export const api = new APIClient();
export default api;
