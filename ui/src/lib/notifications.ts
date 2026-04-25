import { toast } from './toast';

/**
 * Utility functions for showing standardized notifications throughout the app
 */

// Session notifications
export const notify = {
  session: {
    created: (sessionName: string) =>
      toast.success(`Session "${sessionName}" created successfully`),

    deleted: (sessionName: string) =>
      toast.success(`Session "${sessionName}" deleted`),

    hibernated: (sessionName: string) =>
      toast.info(`Session "${sessionName}" is now hibernating`),

    woken: (sessionName: string) =>
      toast.success(`Session "${sessionName}" is waking up`),

    updated: (sessionName: string) =>
      toast.success(`Session "${sessionName}" updated`),

    connected: (sessionName: string) =>
      toast.info(`Connected to "${sessionName}"`),

    disconnected: (sessionName: string) =>
      toast.info(`Disconnected from "${sessionName}"`),
  },

  // User management notifications
  user: {
    created: (username: string) =>
      toast.success(`User "${username}" created successfully`),

    updated: (username: string) =>
      toast.success(`User "${username}" updated`),

    deleted: (username: string) =>
      toast.success(`User "${username}" deleted`),

    loggedIn: (username: string) =>
      toast.success(`Welcome back, ${username}!`),

    loggedOut: () =>
      toast.info('You have been logged out'),
  },

  // Group notifications
  group: {
    created: (groupName: string) =>
      toast.success(`Group "${groupName}" created successfully`),

    updated: (groupName: string) =>
      toast.success(`Group "${groupName}" updated`),

    deleted: (groupName: string) =>
      toast.success(`Group "${groupName}" deleted`),

    memberAdded: (username: string, groupName: string) =>
      toast.success(`${username} added to "${groupName}"`),

    memberRemoved: (username: string, groupName: string) =>
      toast.success(`${username} removed from "${groupName}"`),
  },

  // Template notifications
  template: {
    installed: (templateName: string) =>
      toast.success(`Template "${templateName}" installed`),

    deleted: (templateName: string) =>
      toast.success(`Template "${templateName}" deleted`),

    updated: (templateName: string) =>
      toast.success(`Template "${templateName}" updated`),
  },

  // Repository notifications
  repository: {
    added: (_repoUrl: string) =>
      toast.success('Repository added successfully'),

    removed: (_repoUrl: string) =>
      toast.success('Repository removed'),

    synced: () =>
      toast.success('Repository synchronized'),

    syncStarted: () =>
      toast.info('Synchronizing repositories...'),
  },

  // Quota notifications
  quota: {
    updated: (username: string) =>
      toast.success(`Quota updated for ${username}`),

    deleted: (username: string) =>
      toast.success(`Quota reset for ${username}`),

    warning: (resource: string, percent: number) =>
      toast.warning(`${resource} usage at ${percent}%`),

    exceeded: (resource: string) =>
      toast.error(`${resource} quota exceeded`),
  },

  // Sharing notifications
  sharing: {
    sessionShared: (username: string) =>
      toast.success(`Session shared with ${username}`),

    shareRevoked: (username: string) =>
      toast.success(`Access revoked for ${username}`),

    invitationCreated: () =>
      toast.success('Invitation link created'),

    invitationRevoked: () =>
      toast.success('Invitation revoked'),

    invitationAccepted: (sessionName: string) =>
      toast.success(`You now have access to "${sessionName}"`),

    ownershipTransferred: (newOwner: string) =>
      toast.success(`Ownership transferred to ${newOwner}`),

    collaboratorRemoved: (username: string) =>
      toast.success(`${username} removed from session`),
  },

  // General notifications
  general: {
    saved: () =>
      toast.success('Changes saved successfully'),

    copied: (what: string = 'Text') =>
      toast.success(`${what} copied to clipboard`),

    loading: (what: string = 'Loading') =>
      toast.info(`${what}...`),

    networkError: () =>
      toast.error('Network error. Please check your connection.'),

    permissionDenied: () =>
      toast.error('You do not have permission to perform this action'),

    notFound: (resource: string) =>
      toast.error(`${resource} not found`),
  },
};

export default notify;
