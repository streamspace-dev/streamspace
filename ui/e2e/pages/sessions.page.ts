/**
 * Sessions Page Object
 *
 * Encapsulates interactions with the sessions list page.
 */

import { Page, Locator, expect } from '@playwright/test';

export class SessionsPage {
  readonly page: Page;
  readonly sessionCards: Locator;
  readonly createSessionButton: Locator;
  readonly searchInput: Locator;
  readonly filterDropdown: Locator;
  readonly refreshButton: Locator;
  readonly emptyState: Locator;
  readonly loadingSpinner: Locator;

  constructor(page: Page) {
    this.page = page;
    this.sessionCards = page.locator('[data-testid="session-card"]');
    this.createSessionButton = page.getByRole('button', { name: /new session|create session/i });
    this.searchInput = page.getByPlaceholder(/search/i);
    this.filterDropdown = page.getByLabel(/filter|status/i);
    this.refreshButton = page.getByRole('button', { name: /refresh/i });
    this.emptyState = page.getByText(/no sessions|create your first session/i);
    this.loadingSpinner = page.getByRole('progressbar');
  }

  /**
   * Navigate to sessions page
   */
  async goto(): Promise<void> {
    await this.page.goto('/sessions');
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Wait for sessions to load
   */
  async waitForLoad(): Promise<void> {
    // Wait for loading to finish
    await this.loadingSpinner.waitFor({ state: 'hidden', timeout: 10000 }).catch(() => {});
    // Wait a bit for sessions to render
    await this.page.waitForTimeout(500);
  }

  /**
   * Get count of session cards
   */
  async getSessionCount(): Promise<number> {
    return await this.sessionCards.count();
  }

  /**
   * Get a specific session card by name
   */
  getSessionCard(sessionName: string): Locator {
    return this.page.locator(`[data-testid="session-card"]:has-text("${sessionName}")`);
  }

  /**
   * Click connect button on a session card
   */
  async connectToSession(sessionName: string): Promise<void> {
    const card = this.getSessionCard(sessionName);
    await card.getByRole('button', { name: /connect|open/i }).click();
  }

  /**
   * Click terminate button on a session card
   */
  async terminateSession(sessionName: string): Promise<void> {
    const card = this.getSessionCard(sessionName);
    // Might need to open menu first
    const menuButton = card.getByRole('button', { name: /more|menu/i });
    if (await menuButton.isVisible()) {
      await menuButton.click();
    }
    await this.page.getByRole('menuitem', { name: /terminate|delete/i }).click();
  }

  /**
   * Click hibernate button on a session card
   */
  async hibernateSession(sessionName: string): Promise<void> {
    const card = this.getSessionCard(sessionName);
    await card.getByRole('button', { name: /hibernate|pause/i }).click();
  }

  /**
   * Open the create session dialog/form
   */
  async openCreateDialog(): Promise<void> {
    await this.createSessionButton.click();
  }

  /**
   * Search for sessions
   */
  async search(query: string): Promise<void> {
    await this.searchInput.fill(query);
    await this.page.waitForTimeout(500); // Debounce
  }

  /**
   * Filter by status
   */
  async filterByStatus(status: 'all' | 'running' | 'hibernated' | 'terminated'): Promise<void> {
    await this.filterDropdown.click();
    await this.page.getByRole('option', { name: new RegExp(status, 'i') }).click();
  }

  /**
   * Verify session exists with expected state
   */
  async expectSession(sessionName: string, state?: string): Promise<void> {
    const card = this.getSessionCard(sessionName);
    await expect(card).toBeVisible();
    if (state) {
      await expect(card.getByText(new RegExp(state, 'i'))).toBeVisible();
    }
  }

  /**
   * Verify empty state is shown
   */
  async expectEmptyState(): Promise<void> {
    await expect(this.emptyState).toBeVisible();
  }

  /**
   * Get session state chip text
   */
  async getSessionState(sessionName: string): Promise<string | null> {
    const card = this.getSessionCard(sessionName);
    const stateChip = card.locator('[class*="Chip"]').first();
    return await stateChip.textContent();
  }
}
