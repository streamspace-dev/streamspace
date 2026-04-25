import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import './index.css'

/**
 * Initialize app with optional MSW mocking for tests
 */
async function initApp() {
  // Enable MSW if ?msw=true is in URL or localStorage flag is set
  const enableMSW = window.location.search.includes('msw=true') ||
                    localStorage.getItem('msw-enabled') === 'true';

  if (enableMSW) {
    const { initMSW } = await import('./mocks/init');
    await initMSW();
  }

  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <App />
    </React.StrictMode>,
  );
}

initApp();
