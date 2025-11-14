import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import api, { CreateSessionRequest, CatalogFilters } from '../lib/api';

// ============================================================================
// Session Hooks
// ============================================================================

export function useSessions(user?: string) {
  return useQuery({
    queryKey: ['sessions', user],
    queryFn: () => api.listSessions(user),
    refetchInterval: 5000, // Refetch every 5 seconds for real-time updates
  });
}

export function useSession(id: string) {
  return useQuery({
    queryKey: ['session', id],
    queryFn: () => api.getSession(id),
    enabled: !!id,
    refetchInterval: 3000,
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

export function useDeleteRepository() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => api.deleteRepository(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['repositories'] });
    },
  });
}

// ============================================================================
// Health & Metrics Hooks
// ============================================================================

export function useHealth() {
  return useQuery({
    queryKey: ['health'],
    queryFn: () => api.getHealth(),
    refetchInterval: 30000, // Every 30 seconds
  });
}

export function useMetrics() {
  return useQuery({
    queryKey: ['metrics'],
    queryFn: () => api.getMetrics(),
    refetchInterval: 10000, // Every 10 seconds
  });
}
