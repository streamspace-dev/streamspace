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

export interface UserQuota {
  username: string;
  limits: {
    maxSessions: number;
    maxCPU: string;
    maxMemory: string;
    maxStorage: string;
  };
  used: {
    sessions: number;
    cpu: string;
    memory: string;
    storage: string;
  };
  createdAt: string;
  updatedAt: string;
}

export interface SetQuotaRequest {
  username: string;
  maxSessions?: number;
  maxCPU?: string;
  maxMemory?: string;
  maxStorage?: string;
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

  async login(username: string, password: string): Promise<{ token: string; user: any }> {
    const response = await this.client.post('/auth/login', { username, password });
    return response.data;
  }

  async samlLogin(): Promise<{ redirectUrl: string }> {
    const response = await this.client.get('/saml/login');
    return response.data;
  }

  async samlLogout(): Promise<void> {
    await this.client.post('/saml/logout');
  }

  async getCurrentUser(): Promise<any> {
    const response = await this.client.get('/auth/me');
    return response.data;
  }

  async logout(): Promise<void> {
    await this.client.post('/auth/logout');
    localStorage.removeItem('streamspace_token');
    localStorage.removeItem('streamspace_user');
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
