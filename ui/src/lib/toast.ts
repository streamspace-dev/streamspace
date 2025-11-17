// Toast notification utility using native browser notifications styled as Material UI
import { createRoot } from 'react-dom/client';

export type ToastType = 'success' | 'error' | 'warning' | 'info';

interface ToastOptions {
  duration?: number;
  position?: 'top-right' | 'top-left' | 'bottom-right' | 'bottom-left' | 'top-center' | 'bottom-center';
}

class ToastManager {
  private container: HTMLElement | null = null;
  private toasts: Map<string, { element: HTMLElement; timeout: NodeJS.Timeout }> = new Map();

  private ensureContainer() {
    if (!this.container) {
      this.container = document.createElement('div');
      this.container.id = 'toast-container';
      this.container.style.cssText = `
        position: fixed;
        top: 24px;
        right: 24px;
        z-index: 9999;
        display: flex;
        flex-direction: column;
        gap: 12px;
        pointer-events: none;
      `;
      document.body.appendChild(this.container);
    }
    return this.container;
  }

  private getIcon(type: ToastType): string {
    switch (type) {
      case 'success':
        return '✓';
      case 'error':
        return '✕';
      case 'warning':
        return '⚠';
      case 'info':
        return 'ℹ';
    }
  }

  private getColor(type: ToastType): { bg: string; border: string; text: string } {
    switch (type) {
      case 'success':
        return { bg: '#4caf50', border: '#45a049', text: '#ffffff' };
      case 'error':
        return { bg: '#f44336', border: '#e53935', text: '#ffffff' };
      case 'warning':
        return { bg: '#ff9800', border: '#fb8c00', text: '#ffffff' };
      case 'info':
        return { bg: '#2196f3', border: '#1e88e5', text: '#ffffff' };
    }
  }

  show(message: string, type: ToastType = 'info', options: ToastOptions = {}) {
    const container = this.ensureContainer();
    const id = `toast-${Date.now()}-${Math.random()}`;
    const duration = options.duration || 4000;

    const colors = this.getColor(type);
    const icon = this.getIcon(type);

    const toast = document.createElement('div');
    toast.id = id;
    toast.style.cssText = `
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 12px 16px;
      background: ${colors.bg};
      color: ${colors.text};
      border-left: 4px solid ${colors.border};
      border-radius: 4px;
      box-shadow: 0 2px 8px rgba(0,0,0,0.15);
      font-family: "Roboto", "Helvetica", "Arial", sans-serif;
      font-size: 14px;
      max-width: 400px;
      pointer-events: auto;
      animation: slideIn 0.3s ease-out;
      cursor: pointer;
    `;

    // Create elements safely using DOM API to prevent XSS attacks
    // SECURITY: Using textContent instead of innerHTML to prevent XSS injection
    const iconSpan = document.createElement('span');
    iconSpan.textContent = icon;
    iconSpan.style.cssText = 'font-size: 18px; font-weight: bold;';

    const messageSpan = document.createElement('span');
    messageSpan.textContent = message;  // Safe - no HTML parsing
    messageSpan.style.cssText = 'flex: 1;';

    const closeButton = document.createElement('button');
    closeButton.textContent = '✕';
    closeButton.style.cssText = `
      background: none;
      border: none;
      color: inherit;
      font-size: 16px;
      cursor: pointer;
      padding: 0;
      margin-left: 8px;
      opacity: 0.7;
      transition: opacity 0.2s;
    `;
    closeButton.onmouseover = () => closeButton.style.opacity = '1';
    closeButton.onmouseout = () => closeButton.style.opacity = '0.7';

    toast.appendChild(iconSpan);
    toast.appendChild(messageSpan);
    toast.appendChild(closeButton);

    // Add animation styles if not present
    if (!document.getElementById('toast-animations')) {
      const style = document.createElement('style');
      style.id = 'toast-animations';
      style.textContent = `
        @keyframes slideIn {
          from {
            transform: translateX(100%);
            opacity: 0;
          }
          to {
            transform: translateX(0);
            opacity: 1;
          }
        }
        @keyframes slideOut {
          from {
            transform: translateX(0);
            opacity: 1;
          }
          to {
            transform: translateX(100%);
            opacity: 0;
          }
        }
      `;
      document.head.appendChild(style);
    }

    container.appendChild(toast);

    // Auto-dismiss
    const timeout = setTimeout(() => {
      this.dismiss(id);
    }, duration);

    // Click to dismiss
    const dismissBtn = toast.querySelector('button');
    dismissBtn?.addEventListener('click', (e) => {
      e.stopPropagation();
      this.dismiss(id);
    });

    toast.addEventListener('click', () => {
      this.dismiss(id);
    });

    this.toasts.set(id, { element: toast, timeout });

    return id;
  }

  dismiss(id: string) {
    const toast = this.toasts.get(id);
    if (!toast) return;

    clearTimeout(toast.timeout);

    toast.element.style.animation = 'slideOut 0.3s ease-out';
    setTimeout(() => {
      toast.element.remove();
      this.toasts.delete(id);
    }, 300);
  }

  dismissAll() {
    this.toasts.forEach((_, id) => this.dismiss(id));
  }

  success(message: string, options?: ToastOptions) {
    return this.show(message, 'success', options);
  }

  error(message: string, options?: ToastOptions) {
    return this.show(message, 'error', { ...options, duration: options?.duration || 6000 });
  }

  warning(message: string, options?: ToastOptions) {
    return this.show(message, 'warning', options);
  }

  info(message: string, options?: ToastOptions) {
    return this.show(message, 'info', options);
  }
}

export const toast = new ToastManager();
