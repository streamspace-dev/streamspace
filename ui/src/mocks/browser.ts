/**
 * MSW Browser Setup
 *
 * Configures the mock service worker for browser environments.
 * Used during development and testing to intercept API requests.
 */

import { setupWorker } from 'msw/browser';
import { handlers } from './handlers';

// Create the worker instance
export const worker = setupWorker(...handlers);

// Export for use in tests and development
export { handlers };
