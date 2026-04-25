/**
 * MSW Initialization
 *
 * Conditionally starts MSW based on environment.
 * Call this at app startup to enable API mocking.
 */

export async function initMSW(): Promise<void> {
  // Only run in development or when explicitly enabled
  if (import.meta.env.MODE !== 'development' && !import.meta.env.VITE_ENABLE_MOCKS) {
    return;
  }

  // Check if running in test mode (Playwright sets this)
  const isTestMode = window.location.search.includes('msw=true') ||
                     localStorage.getItem('msw-enabled') === 'true' ||
                     import.meta.env.VITE_ENABLE_MOCKS === 'true';

  if (!isTestMode && import.meta.env.MODE === 'development') {
    // In development, only enable if explicitly requested
    console.log('MSW: Development mode - not started (add ?msw=true to enable)');
    return;
  }

  try {
    const { worker } = await import('./browser');

    await worker.start({
      onUnhandledRequest: 'bypass', // Don't warn about unhandled requests
      serviceWorker: {
        url: '/mockServiceWorker.js',
      },
    });

    console.log('MSW: Mock Service Worker started');
  } catch (error) {
    console.error('MSW: Failed to start Mock Service Worker', error);
  }
}
