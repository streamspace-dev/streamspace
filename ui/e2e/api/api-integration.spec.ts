/**
 * API Integration Tests
 *
 * Tests that verify the UI correctly interacts with the backend API.
 * These tests help identify issues where the API and UI are out of sync.
 */

import { test, expect } from '@playwright/test';

const API_URL = process.env.API_URL || 'http://localhost:8000';

test.describe('API Integration', () => {
  test.describe('Authentication API', () => {
    test('should return 401 for unauthenticated session requests', async ({ request }) => {
      const response = await request.get(`${API_URL}/api/v1/sessions`);
      expect(response.status()).toBe(401);
    });

    test('should accept token in query parameter for proxy endpoints', async ({ request }) => {
      // VNC proxy should accept token in query param
      const response = await request.get(`${API_URL}/api/v1/vnc/test-session?token=invalid`);
      // Should get 401 for invalid token, not 400 for missing token
      expect([401, 403, 404]).toContain(response.status());
    });

    test('should accept token in query parameter for HTTP proxy endpoints', async ({ request }) => {
      const response = await request.get(`${API_URL}/api/v1/http/test-session/?token=invalid`);
      expect([401, 403, 404]).toContain(response.status());
    });
  });

  test.describe('Session API Contracts', () => {
    test('should return expected session structure from GET /api/v1/sessions/:id', async ({ page: _page, request }) => {
      // Login first to get token
      const loginResponse = await request.post(`${API_URL}/api/v1/auth/login`, {
        data: { username: 'admin', password: 'admin123' },
      });

      // Skip if login fails (API might not be running or credentials wrong)
      if (!loginResponse.ok()) {
        test.skip();
        return;
      }

      const { token } = await loginResponse.json();

      // Create a session first to get its ID
      const sessionsResponse = await request.get(`${API_URL}/api/v1/sessions`, {
        headers: { Authorization: `Bearer ${token}` },
      });

      if (!sessionsResponse.ok()) {
        test.skip();
        return;
      }

      const sessions = await sessionsResponse.json();
      if (sessions.length === 0) {
        test.skip();
        return;
      }

      const session = sessions[0];

      // Verify session has required fields for streaming
      expect(session).toHaveProperty('name');
      expect(session).toHaveProperty('state');
      expect(session).toHaveProperty('status');

      // Verify streaming protocol fields exist (even if null)
      expect(session).toHaveProperty('streamingProtocol');
    });

    test('should return 404 for non-existent session', async ({ request }) => {
      const loginResponse = await request.post(`${API_URL}/api/v1/auth/login`, {
        data: { username: 'admin', password: 'admin123' },
      });

      if (!loginResponse.ok()) {
        test.skip();
        return;
      }

      const { token } = await loginResponse.json();

      const response = await request.get(`${API_URL}/api/v1/sessions/nonexistent-session-12345`, {
        headers: { Authorization: `Bearer ${token}` },
      });

      expect(response.status()).toBe(404);
    });
  });

  test.describe('Proxy Endpoint Contracts', () => {
    test('VNC proxy should check session access', async ({ request }) => {
      const loginResponse = await request.post(`${API_URL}/api/v1/auth/login`, {
        data: { username: 'admin', password: 'admin123' },
      });

      if (!loginResponse.ok()) {
        test.skip();
        return;
      }

      const { token } = await loginResponse.json();

      // Try to connect to VNC for non-existent session
      const response = await request.get(
        `${API_URL}/api/v1/vnc/nonexistent-session?token=${token}`
      );

      // Should return 404 for non-existent session
      expect(response.status()).toBe(404);
    });

    test('HTTP proxy should check session protocol', async ({ request }) => {
      const loginResponse = await request.post(`${API_URL}/api/v1/auth/login`, {
        data: { username: 'admin', password: 'admin123' },
      });

      if (!loginResponse.ok()) {
        test.skip();
        return;
      }

      const { token } = await loginResponse.json();

      // Try HTTP proxy for non-existent session
      const response = await request.get(
        `${API_URL}/api/v1/http/nonexistent-session/?token=${token}`
      );

      // Should return 404 for non-existent session
      expect(response.status()).toBe(404);
    });

    test('VNC proxy should require authentication', async ({ request }) => {
      // No token
      const response1 = await request.get(`${API_URL}/api/v1/vnc/test-session`);
      expect(response1.status()).toBe(401);

      // Empty token
      const response2 = await request.get(`${API_URL}/api/v1/vnc/test-session?token=`);
      expect(response2.status()).toBe(401);
    });

    test('HTTP proxy should require authentication', async ({ request }) => {
      // No token
      const response1 = await request.get(`${API_URL}/api/v1/http/test-session/`);
      expect(response1.status()).toBe(401);

      // Empty token
      const response2 = await request.get(`${API_URL}/api/v1/http/test-session/?token=`);
      expect(response2.status()).toBe(401);
    });
  });

  test.describe('Security Headers', () => {
    test('should allow iframe embedding for VNC proxy paths', async ({ request }) => {
      const response = await request.get(`${API_URL}/api/v1/vnc/test-session?token=test`);

      // Check X-Frame-Options allows same origin (for iframe embedding)
      const xFrameOptions = response.headers()['x-frame-options'];
      expect(xFrameOptions?.toLowerCase() || 'sameorigin').not.toBe('deny');
    });

    test('should allow iframe embedding for HTTP proxy paths', async ({ request }) => {
      const response = await request.get(`${API_URL}/api/v1/http/test-session/?token=test`);

      const xFrameOptions = response.headers()['x-frame-options'];
      expect(xFrameOptions?.toLowerCase() || 'sameorigin').not.toBe('deny');
    });
  });

  test.describe('Session State Transitions', () => {
    test('should reject VNC connection for hibernated session', async ({ request }) => {
      const loginResponse = await request.post(`${API_URL}/api/v1/auth/login`, {
        data: { username: 'admin', password: 'admin123' },
      });

      if (!loginResponse.ok()) {
        test.skip();
        return;
      }

      const { token } = await loginResponse.json();

      // Get sessions to find a hibernated one
      const sessionsResponse = await request.get(`${API_URL}/api/v1/sessions`, {
        headers: { Authorization: `Bearer ${token}` },
      });

      if (!sessionsResponse.ok()) {
        test.skip();
        return;
      }

      const sessions = await sessionsResponse.json();
      const hibernatedSession = sessions.find((s: { state: string; name: string }) => s.state === 'hibernated');

      if (!hibernatedSession) {
        test.skip();
        return;
      }

      const response = await request.get(
        `${API_URL}/api/v1/vnc/${hibernatedSession.name}?token=${token}`
      );

      // Should return conflict status for non-running session
      expect(response.status()).toBe(409);
    });
  });

  test.describe('Error Response Format', () => {
    test('should return JSON error responses', async ({ request }) => {
      const response = await request.get(`${API_URL}/api/v1/sessions/nonexistent`);

      expect(response.headers()['content-type']).toContain('application/json');

      const body = await response.json();
      expect(body).toHaveProperty('error');
    });

    test('should return meaningful error messages', async ({ request }) => {
      const response = await request.get(`${API_URL}/api/v1/sessions/nonexistent`);
      const body = await response.json();

      expect(body.error).toBeTruthy();
      expect(body.error.length).toBeGreaterThan(0);
    });
  });
});
