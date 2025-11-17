import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import api, { CreateSessionRequest, CatalogFilters } from '../lib/api';

// ============================================================================
// Session Hooks
// ============================================================================

export function useSessions(user?: string) {
  return useQuery({
    queryKey: ['sessions', user],
    queryFn: () => api.listSessions(user),
    // Polling disabled - use WebSocket for real-time updates via useSessionsWebSocket
  });
}

export function useSession(id: string) {
  return useQuery({
    queryKey: ['session', id],
    queryFn: () => api.getSession(id),
    enabled: !!id,
    // Polling disabled - use WebSocket for real-time updates via useSessionsWebSocket
  });
}

export function useCreateSession() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateSessionRequest) => api.createSession(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sessions'] });
    },
  });
}

export function useUpdateSessionState() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, state }: { id: string; state: 'running' | 'hibernated' | 'terminated' }) =>
      api.updateSessionState(id, state),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['sessions'] });
      queryClient.invalidateQueries({ queryKey: ['session', variables.id] });
    },
  });
}

export function useDeleteSession() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => api.deleteSession(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sessions'] });
    },
  });
}

export function useConnectSession() {
  return useMutation({
    mutationFn: ({ id, user }: { id: string; user: string }) => api.connectSession(id, user),
  });
}

// ============================================================================
// Template Hooks
// ============================================================================

export function useTemplates(category?: string) {
  return useQuery({
    queryKey: ['templates', category],
    queryFn: () => api.listTemplates(category),
  });
}

export function useTemplate(id: string) {
  return useQuery({
    queryKey: ['template', id],
    queryFn: () => api.getTemplate(id),
    enabled: !!id,
  });
}

export function useDeleteTemplate() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => api.deleteTemplate(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['templates'] });
    },
  });
}

// ============================================================================
// Catalog Hooks
// ============================================================================

export function useCatalogTemplates(filters?: CatalogFilters) {
  return useQuery({
    queryKey: ['catalog', filters],
    queryFn: () => api.listCatalogTemplates(filters),
  });
}

export function useInstallCatalogTemplate() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => api.installCatalogTemplate(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['templates'] });
    },
  });
}

// ============================================================================
// Repository Hooks
// ============================================================================

export function useRepositories() {
  return useQuery({
    queryKey: ['repositories'],
    queryFn: () => api.listRepositories(),
  });
}

export function useAddRepository() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: { name: string; url: string; branch?: string; authType?: string }) =>
      api.addRepository(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['repositories'] });
    },
  });
}

export function useSyncRepository() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => api.syncRepository(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['repositories'] });
    },
  });
}

export function useSyncAllRepositories() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => api.syncAllRepositories(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['repositories'] });
      queryClient.invalidateQueries({ queryKey: ['catalog'] });
    },
  });
}

export function useUpdateRepository() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: any }) => api.updateRepository(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['repositories'] });
    },
  });
}

export function useDeleteRepository() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => api.deleteRepository(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['repositories'] });
    },
  });
}

export function useRepositoryStats(id: number) {
  return useQuery({
    queryKey: ['repository-stats', id],
    queryFn: () => api.getRepositoryStats(id),
    enabled: !!id,
  });
}

// ============================================================================
// Health & Metrics Hooks
// ============================================================================

export function useHealth() {
  return useQuery({
    queryKey: ['health'],
    queryFn: () => api.getHealth(),
    // Polling disabled to reduce unnecessary API calls - health checks are passive
  });
}

export function useMetrics() {
  return useQuery({
    queryKey: ['metrics'],
    queryFn: () => api.getMetrics(),
    // Polling disabled - use WebSocket for real-time updates via useMetricsWebSocket
  });
}
