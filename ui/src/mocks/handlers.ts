/**
 * MSW Request Handlers
 *
 * Defines mock API handlers for testing without a real backend.
 * These handlers intercept requests at the service worker level,
 * bypassing Vite's proxy configuration.
 */

import { http, HttpResponse } from 'msw';

// Mock data
export const MOCK_USERS = {
  admin: {
    user_id: 'admin',
    username: 'admin',
    email: 'admin@streamspace.local',
    role: 'admin',
    org_id: 'default-org',
  },
  testuser: {
    user_id: 'testuser',
    username: 'testuser',
    email: 'testuser@streamspace.local',
    role: 'user',
    org_id: 'default-org',
  },
};

export const MOCK_SESSIONS = {
  running: {
    name: 'test-session-running',
    user: 'admin',
    template: 'chromium',
    state: 'running',
    platform: 'kubernetes',
    agent_id: 'k8s-agent-1',
    streamingProtocol: 'selkies',
    streamingPort: 3000,
    streamingPath: '/websockify',
    status: {
      phase: 'Running',
      url: 'http://test-session-running.streamspace.svc.cluster.local:3000',
      podName: 'test-session-running-abc123',
    },
    activeConnections: 0,
    resources: { cpu: '500m', memory: '2Gi' },
    created_at: new Date().toISOString(),
    last_activity: new Date().toISOString(),
  },
  hibernated: {
    name: 'test-session-hibernated',
    user: 'admin',
    template: 'firefox',
    state: 'hibernated',
    platform: 'kubernetes',
    agent_id: 'k8s-agent-1',
    streamingProtocol: 'vnc',
    streamingPort: 5900,
    status: {
      phase: 'Hibernated',
    },
    activeConnections: 0,
    resources: { cpu: '500m', memory: '2Gi' },
    created_at: new Date().toISOString(),
    last_activity: new Date().toISOString(),
  },
  vnc: {
    name: 'test-session-vnc',
    user: 'admin',
    template: 'firefox',
    state: 'running',
    platform: 'kubernetes',
    agent_id: 'k8s-agent-1',
    streamingProtocol: 'vnc',
    streamingPort: 5900,
    status: {
      phase: 'Running',
      url: 'http://test-session-vnc.streamspace.svc.cluster.local:5900',
      podName: 'test-session-vnc-def456',
    },
    activeConnections: 1,
    resources: { cpu: '500m', memory: '2Gi' },
    created_at: new Date().toISOString(),
    last_activity: new Date().toISOString(),
  },
};

export const MOCK_TEMPLATES = [
  {
    name: 'chromium',
    displayName: 'Chromium Browser',
    description: 'Chromium web browser with Selkies WebRTC streaming',
    category: 'browsers',
    icon: '/icons/chromium.svg',
    baseImage: 'lscr.io/linuxserver/chromium:latest',
    defaultResources: { memory: '2Gi', cpu: '500m' },
  },
  {
    name: 'firefox',
    displayName: 'Firefox Browser',
    description: 'Firefox web browser',
    category: 'browsers',
    icon: '/icons/firefox.svg',
    baseImage: 'lscr.io/linuxserver/firefox:latest',
    defaultResources: { memory: '2Gi', cpu: '500m' },
  },
  {
    name: 'vscode',
    displayName: 'VS Code',
    description: 'Visual Studio Code editor',
    category: 'development',
    icon: '/icons/vscode.svg',
    baseImage: 'lscr.io/linuxserver/code-server:latest',
    defaultResources: { memory: '4Gi', cpu: '1000m' },
  },
];

export const MOCK_AGENTS = [
  {
    agent_id: 'k8s-agent-1',
    name: 'K8s Agent 1',
    platform: 'kubernetes',
    region: 'us-east-1',
    status: 'online',
    capacity: { maxCpu: '64', maxMemory: '256Gi', maxSessions: 100 },
    current: { activeSessions: 5, cpuUsed: '2500m', memoryUsed: '10Gi' },
    last_heartbeat: new Date().toISOString(),
  },
];

// Generate a mock JWT token
function generateMockToken(user: typeof MOCK_USERS.admin): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const payload = btoa(JSON.stringify({
    user_id: user.user_id,
    username: user.username,
    email: user.email,
    role: user.role,
    org_id: user.org_id,
    exp: Math.floor(Date.now() / 1000) + 86400, // 24 hours
    iat: Math.floor(Date.now() / 1000),
  }));
  const signature = btoa('mock-signature');
  return `${header}.${payload}.${signature}`;
}

/**
 * API request handlers
 */
export const handlers = [
  // Auth endpoints
  http.post('/api/v1/auth/login', async ({ request }) => {
    const body = await request.json() as { username: string; password: string };

    if (body.username === 'admin' && body.password === 'admin123') {
      return HttpResponse.json({
        token: generateMockToken(MOCK_USERS.admin),
        user: MOCK_USERS.admin,
      });
    }

    if (body.username === 'testuser' && body.password === 'testuser123') {
      return HttpResponse.json({
        token: generateMockToken(MOCK_USERS.testuser),
        user: MOCK_USERS.testuser,
      });
    }

    return HttpResponse.json(
      { error: 'Invalid credentials' },
      { status: 401 }
    );
  }),

  http.get('/api/v1/auth/me', ({ request }) => {
    const authHeader = request.headers.get('Authorization');
    if (!authHeader?.startsWith('Bearer ')) {
      return HttpResponse.json({ error: 'Unauthorized' }, { status: 401 });
    }
    return HttpResponse.json(MOCK_USERS.admin);
  }),

  http.post('/api/v1/auth/logout', () => {
    return HttpResponse.json({ message: 'Logged out successfully' });
  }),

  // Sessions endpoints
  http.get('/api/v1/sessions', () => {
    return HttpResponse.json([
      MOCK_SESSIONS.running,
      MOCK_SESSIONS.hibernated,
      MOCK_SESSIONS.vnc,
    ]);
  }),

  http.get('/api/v1/sessions/:sessionId', ({ params }) => {
    const { sessionId } = params;

    // Find matching session
    const session = Object.values(MOCK_SESSIONS).find(s => s.name === sessionId);
    if (session) {
      return HttpResponse.json(session);
    }

    // Return a running session with the requested ID
    return HttpResponse.json({
      ...MOCK_SESSIONS.running,
      name: sessionId,
    });
  }),

  http.post('/api/v1/sessions', async ({ request }) => {
    const body = await request.json() as { template: string; name?: string };
    const newSession = {
      ...MOCK_SESSIONS.running,
      name: body.name || `session-${Date.now()}`,
      template: body.template,
      created_at: new Date().toISOString(),
    };
    return HttpResponse.json(newSession, { status: 201 });
  }),

  http.delete('/api/v1/sessions/:sessionId', () => {
    return HttpResponse.json({ status: 'terminated' });
  }),

  http.post('/api/v1/sessions/:sessionId/connect', () => {
    return HttpResponse.json({
      connectionId: `conn-${Date.now()}`,
      sessionUrl: 'http://test.local:3000',
      state: 'running',
      message: 'Connected successfully',
    });
  }),

  http.post('/api/v1/sessions/:sessionId/disconnect', () => {
    return HttpResponse.json({ status: 'disconnected' });
  }),

  http.post('/api/v1/sessions/:sessionId/heartbeat', () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.post('/api/v1/sessions/:sessionId/hibernate', () => {
    return HttpResponse.json({ status: 'hibernating' });
  }),

  http.post('/api/v1/sessions/:sessionId/resume', () => {
    return HttpResponse.json({
      ...MOCK_SESSIONS.running,
      state: 'running',
    });
  }),

  // Templates endpoints
  http.get('/api/v1/templates', () => {
    return HttpResponse.json(MOCK_TEMPLATES);
  }),

  http.get('/api/v1/templates/:templateId', ({ params }) => {
    const { templateId } = params;
    const template = MOCK_TEMPLATES.find(t => t.name === templateId);
    if (template) {
      return HttpResponse.json(template);
    }
    return HttpResponse.json({ error: 'Template not found' }, { status: 404 });
  }),

  // Agents endpoints
  http.get('/api/v1/agents', () => {
    return HttpResponse.json(MOCK_AGENTS);
  }),

  // VNC proxy (returns session info for HTTP-based protocols)
  http.get('/api/v1/vnc/:sessionId', ({ params, request }) => {
    const url = new URL(request.url);
    const token = url.searchParams.get('token');

    if (!token) {
      return HttpResponse.json({ error: 'Unauthorized' }, { status: 401 });
    }

    const { sessionId } = params;
    const session = Object.values(MOCK_SESSIONS).find(s => s.name === sessionId);

    if (!session) {
      return HttpResponse.json({ error: 'Session not found' }, { status: 404 });
    }

    if (session.state !== 'running') {
      return HttpResponse.json(
        { error: `Session is not running (state: ${session.state})` },
        { status: 409 }
      );
    }

    // For HTTP-based protocols, return session info
    if (['selkies', 'kasm', 'guacamole'].includes(session.streamingProtocol || '')) {
      return HttpResponse.json({
        type: 'http_session',
        session_id: sessionId,
        protocol: session.streamingProtocol,
        url: session.status.url,
        port: session.streamingPort,
        path: session.streamingPath,
      });
    }

    // For VNC, we'd normally upgrade to WebSocket
    return HttpResponse.json({ error: 'WebSocket upgrade required' }, { status: 426 });
  }),

  // HTTP proxy for Selkies/Kasm/Guacamole
  http.all('/api/v1/http/:sessionId/*', ({ params, request }) => {
    const url = new URL(request.url);
    const token = url.searchParams.get('token');

    if (!token) {
      return HttpResponse.json({ error: 'Unauthorized' }, { status: 401 });
    }

    const { sessionId } = params;
    const session = Object.values(MOCK_SESSIONS).find(s => s.name === sessionId);

    if (!session) {
      return HttpResponse.json({ error: 'Session not found' }, { status: 404 });
    }

    // Return mock streaming content
    return new HttpResponse(
      `<!DOCTYPE html>
<html>
<head><title>StreamSpace Session - ${sessionId}</title></head>
<body data-testid="stream-content">
  <h1>Mock Stream Content</h1>
  <p>Session: ${sessionId}</p>
  <p>Protocol: ${session.streamingProtocol}</p>
  <div id="stream-container"></div>
</body>
</html>`,
      {
        status: 200,
        headers: {
          'Content-Type': 'text/html',
          'X-Frame-Options': 'SAMEORIGIN',
        },
      }
    );
  }),

  // Dashboard metrics
  http.get('/api/v1/dashboard/metrics', () => {
    return HttpResponse.json({
      activeSessions: 3,
      totalUsage: '45.2 hours',
      costEstimate: '$12.50',
      agentsOnline: 1,
    });
  }),

  // Users (admin)
  http.get('/api/v1/users', () => {
    return HttpResponse.json([MOCK_USERS.admin, MOCK_USERS.testuser]);
  }),

  // System metrics (admin)
  http.get('/api/v1/system/metrics', () => {
    return HttpResponse.json({
      cpu: '25%',
      memory: '45%',
      disk: '60%',
      uptime: '7 days',
    });
  }),
];
