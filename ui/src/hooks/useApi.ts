import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import api, { CreateSessionRequest, CatalogFilters, InstallApplicationRequest, UpdateApplicationRequest, AddGroupAccessRequest } from '../lib/api';

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
    mutationFn: ({ id, data }: { id: number; data: Partial<{ name: string; url: string; branch: string; authType: string; authSecret: string }> }) => api.updateRepository(id, data),
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

// ============================================================================
// Quota Hooks
// ============================================================================

export function useCurrentUserQuota() {
  return useQuery({
    queryKey: ['current-user-quota'],
    queryFn: () => api.getCurrentUserQuota(),
    // Polling disabled - quota data is relatively static, refresh on user actions only
  });
}

// ============================================================================
// Scheduling Hooks
// ============================================================================

export function useScheduledSessions() {
  return useQuery({
    queryKey: ['scheduled-sessions'],
    queryFn: () => api.listScheduledSessions(),
    select: (data) => data.schedules,
    // Polling disabled - use WebSocket for real-time updates via useScheduleEvents
  });
}

export function useCalendarIntegrations() {
  return useQuery({
    queryKey: ['calendar-integrations'],
    queryFn: () => api.listCalendarIntegrations(),
    select: (data) => data.integrations,
    // Polling disabled - calendar integrations are relatively static
  });
}

// ============================================================================
// Security Settings Hooks
// ============================================================================

export function useMFAMethods() {
  return useQuery({
    queryKey: ['mfa-methods'],
    queryFn: () => api.listMFAMethods(),
    select: (data) => data.methods,
    // Polling disabled - MFA methods are relatively static
  });
}

export function useIPWhitelist() {
  return useQuery({
    queryKey: ['ip-whitelist'],
    queryFn: () => api.listIPWhitelist(),
    select: (data) => data.entries,
    // Polling disabled - IP whitelist is static configuration
  });
}

export function useSecurityAlerts() {
  return useQuery({
    queryKey: ['security-alerts'],
    queryFn: () => api.getSecurityAlerts(),
    select: (data) => data.alerts,
    // Polling disabled - use WebSocket for real-time updates via useSecurityAlertEvents
  });
}

// ============================================================================
// Plugin Hooks
// ============================================================================

export function useInstalledPlugins() {
  return useQuery({
    queryKey: ['installed-plugins'],
    queryFn: () => api.listInstalledPlugins(),
    // Polling disabled - use WebSocket for real-time updates via usePluginEvents
  });
}

export function useBrowsePlugins(filters?: CatalogFilters) {
  return useQuery({
    queryKey: ['browse-plugins', filters],
    queryFn: () => api.browsePlugins(filters),
    // Polling disabled - catalog data is relatively static
  });
}

// ============================================================================
// Installed Applications Hooks
// ============================================================================

export function useApplications(enabledOnly?: boolean) {
  return useQuery({
    queryKey: ['applications', enabledOnly],
    queryFn: () => api.listApplications(enabledOnly),
    select: (data) => data.applications,
  });
}

export function useApplication(id: string) {
  return useQuery({
    queryKey: ['application', id],
    queryFn: () => api.getApplication(id),
    enabled: !!id,
  });
}

export function useUserApplications() {
  return useQuery({
    queryKey: ['user-applications'],
    queryFn: () => api.getUserApplications(),
    select: (data) => data.applications,
  });
}

export function useInstallApplication() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: InstallApplicationRequest) => api.installApplication(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['applications'] });
    },
  });
}

export function useUpdateApplication() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateApplicationRequest }) =>
      api.updateApplication(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['applications'] });
      queryClient.invalidateQueries({ queryKey: ['application', variables.id] });
    },
  });
}

export function useDeleteApplication() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => api.deleteApplication(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['applications'] });
    },
  });
}

export function useSetApplicationEnabled() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      api.setApplicationEnabled(id, enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['applications'] });
    },
  });
}

export function useApplicationGroups(id: string) {
  return useQuery({
    queryKey: ['application-groups', id],
    queryFn: () => api.getApplicationGroups(id),
    select: (data) => data.groups,
    enabled: !!id,
  });
}

export function useAddApplicationGroupAccess() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: AddGroupAccessRequest }) =>
      api.addApplicationGroupAccess(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['application-groups', variables.id] });
      queryClient.invalidateQueries({ queryKey: ['application', variables.id] });
    },
  });
}

export function useRemoveApplicationGroupAccess() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, groupId }: { id: string; groupId: string }) =>
      api.removeApplicationGroupAccess(id, groupId),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['application-groups', variables.id] });
      queryClient.invalidateQueries({ queryKey: ['application', variables.id] });
    },
  });
}

// ============================================================================
// Shared Sessions Hooks
// ============================================================================

export function useSharedSessions(userId?: string) {
  return useQuery({
    queryKey: ['shared-sessions', userId],
    queryFn: () => api.listSharedSessions(userId!),
    enabled: !!userId,
    // Polling disabled - use WebSocket for real-time updates via useSessionsWebSocket
  });
}
