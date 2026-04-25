/**
 * Session Viewer Page Object
 *
 * Encapsulates interactions with the session streaming viewer page.
 * This is critical for testing VNC and Selkies streaming functionality.
 */

import { Page, Locator, expect, FrameLocator } from '@playwright/test';

export class SessionViewerPage {
  readonly page: Page;
  readonly toolbar: Locator;
  readonly sessionTitle: Locator;
  readonly connectionStatus: Locator;
  readonly fullscreenButton: Locator;
  readonly refreshButton: Locator;
  readonly closeButton: Locator;
  readonly infoButton: Locator;
  readonly shareButton: Locator;
  readonly streamingIframe: Locator;
  readonly loadingSpinner: Locator;
  readonly errorAlert: Locator;
  readonly connectionChip: Locator;
  readonly infoDialog: Locator;

  constructor(page: Page) {
    this.page = page;
    this.toolbar = page.locator('header, [class*="AppBar"]');
    this.sessionTitle = page.locator('header h6, [class*="AppBar"] h6');
    this.connectionStatus = page.locator('[class*="WebSocketStatus"], [data-testid="connection-status"]');
    this.fullscreenButton = page.getByRole('button', { name: /fullscreen/i });
    this.refreshButton = page.getByRole('button', { name: /refresh/i });
    this.closeButton = page.getByRole('button', { name: /close/i });
    this.infoButton = page.getByRole('button', { name: /info/i });
    this.shareButton = page.getByRole('button', { name: /share/i });
    this.streamingIframe = page.locator('iframe[title^="Session"]');
    this.loadingSpinner = page.getByRole('progressbar');
    this.errorAlert = page.getByRole('alert');
    this.connectionChip = page.locator('[class*="Chip"]:has-text("connection")');
    this.infoDialog = page.getByRole('dialog');
  }

  /**
   * Navigate to session viewer for a specific session
   */
  async goto(sessionId: string): Promise<void> {
    await this.page.goto(`/sessions/${sessionId}/view`);
  }

  /**
   * Wait for the viewer to load
   */
  async waitForLoad(): Promise<void> {
    // Wait for loading spinner to disappear
    await this.loadingSpinner.waitFor({ state: 'hidden', timeout: 30000 }).catch(() => {});
    // Wait for either iframe or error
    await Promise.race([
      this.streamingIframe.waitFor({ state: 'visible', timeout: 10000 }),
      this.errorAlert.waitFor({ state: 'visible', timeout: 10000 }),
    ]).catch(() => {});
  }

  /**
   * Verify the streaming iframe is visible
   */
  async expectStreamingVisible(): Promise<void> {
    await expect(this.streamingIframe).toBeVisible();
  }

  /**
   * Verify an error is displayed
   */
  async expectError(message?: string): Promise<void> {
    await expect(this.errorAlert).toBeVisible();
    if (message) {
      await expect(this.errorAlert).toContainText(message);
    }
  }

  /**
   * Get the iframe source URL
   */
  async getIframeSrc(): Promise<string | null> {
    return await this.streamingIframe.getAttribute('src');
  }

  /**
   * Verify the iframe src matches expected protocol pattern
   */
  async expectProtocol(protocol: 'vnc' | 'selkies' | 'http'): Promise<void> {
    const src = await this.getIframeSrc();
    expect(src).not.toBeNull();

    if (protocol === 'vnc') {
      expect(src).toContain('/vnc-viewer/');
    } else if (protocol === 'selkies' || protocol === 'http') {
      expect(src).toContain('/api/v1/http/');
    }
  }

  /**
   * Verify token is present in iframe src
   */
  async expectTokenInUrl(): Promise<void> {
    const src = await this.getIframeSrc();
    expect(src).not.toBeNull();
    expect(src).toContain('token=');
    // Verify token is not empty
    const tokenMatch = src?.match(/token=([^&]+)/);
    expect(tokenMatch).not.toBeNull();
    expect(tokenMatch![1]).not.toBe('');
    expect(tokenMatch![1]).not.toBe('null');
    expect(tokenMatch![1]).not.toBe('undefined');
  }

  /**
   * Click fullscreen button
   */
  async toggleFullscreen(): Promise<void> {
    await this.fullscreenButton.click();
  }

  /**
   * Click refresh button
   */
  async refresh(): Promise<void> {
    await this.refreshButton.click();
    await this.waitForLoad();
  }

  /**
   * Click close button to go back to sessions
   */
  async close(): Promise<void> {
    await this.closeButton.click();
    await expect(this.page).toHaveURL(/\/sessions/);
  }

  /**
   * Open session info dialog
   */
  async openInfoDialog(): Promise<void> {
    await this.infoButton.click();
    await expect(this.infoDialog).toBeVisible();
  }

  /**
   * Verify session info is displayed correctly
   */
  async expectSessionInfo(expectedInfo: {
    name?: string;
    template?: string;
    state?: string;
    platform?: string;
    agentId?: string;
  }): Promise<void> {
    await this.openInfoDialog();

    if (expectedInfo.name) {
      await expect(this.infoDialog.getByText(expectedInfo.name)).toBeVisible();
    }
    if (expectedInfo.template) {
      await expect(this.infoDialog.getByText(expectedInfo.template)).toBeVisible();
    }
    if (expectedInfo.state) {
      await expect(this.infoDialog.getByText(expectedInfo.state)).toBeVisible();
    }
    if (expectedInfo.platform) {
      await expect(this.infoDialog.getByText(expectedInfo.platform)).toBeVisible();
    }
    if (expectedInfo.agentId) {
      await expect(this.infoDialog.getByText(expectedInfo.agentId)).toBeVisible();
    }

    // Close dialog
    await this.page.getByRole('button', { name: /close/i }).click();
  }

  /**
   * Get the frame locator for the streaming iframe (for inspecting iframe content)
   */
  getStreamingFrame(): FrameLocator {
    return this.page.frameLocator('iframe[title^="Session"]');
  }

  /**
   * Verify streaming content is loaded in iframe
   * This checks if the iframe has actual content (not blank)
   */
  async expectStreamingContent(): Promise<void> {
    const frame = this.getStreamingFrame();
    // Check for VNC canvas or Selkies content
    await Promise.race([
      frame.locator('canvas').waitFor({ state: 'visible', timeout: 10000 }),
      frame.locator('body[data-testid="stream-content"]').waitFor({ state: 'visible', timeout: 10000 }),
      frame.locator('#vnc-container').waitFor({ state: 'visible', timeout: 10000 }),
    ]).catch(() => {
      // If none found, check for any body content
      return frame.locator('body').waitFor({ state: 'visible', timeout: 5000 });
    });
  }

  /**
   * Verify connection count is displayed
   */
  async expectConnectionCount(count: number): Promise<void> {
    await expect(this.connectionChip).toContainText(`${count} connection`);
  }
}
