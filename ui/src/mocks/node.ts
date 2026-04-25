/**
 * MSW Node Setup
 *
 * Configures the mock service worker for Node.js environments.
 * Used in Playwright tests to intercept API requests.
 */

import { setupServer } from 'msw/node';
import { handlers } from './handlers';

// Create the server instance
export const server = setupServer(...handlers);

// Export handlers for custom overrides
export { handlers };
