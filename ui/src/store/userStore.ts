import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User, LoginResponse } from '../lib/api';

interface AuthState {
  // Authentication state
  user: User | null;
  token: string | null;
  expiresAt: string | null;
  isAuthenticated: boolean;

  // Actions
  setAuth: (loginResponse: LoginResponse) => void;
  updateUser: (user: User) => void;
  clearAuth: () => void;
  isTokenExpired: () => boolean;
}

export const useUserStore = create<AuthState>()(
  persist(
    (set, get) => ({
      // Initial state
      user: null,
      token: null,
      expiresAt: null,
      isAuthenticated: false,

      // Set authentication from login response.
      //
      // Defensive default for expiresAt: TypeScript marks LoginResponse.expiresAt
      // as required, but TS type contracts aren't enforced at the network
      // boundary. A backend (or MSW mock) returning {token, user} without
      // expiresAt would otherwise leave the field undefined, isTokenExpired()
      // would return true on the next render, and the route guard would bounce
      // the user back to /login with no error shown — silent broken auth.
      // Default to 24h ahead so the session is at least usable until the next
      // /auth/me call refreshes state.
      setAuth: (loginResponse: LoginResponse) =>
        set({
          user: loginResponse.user,
          token: loginResponse.token,
          expiresAt:
            loginResponse.expiresAt ??
            new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
          isAuthenticated: true,
        }),

      // Update user information (for profile updates)
      updateUser: (user: User) =>
        set({ user }),

      // Clear authentication (logout)
      clearAuth: () =>
        set({
          user: null,
          token: null,
          expiresAt: null,
          isAuthenticated: false,
        }),

      // Check if token is expired
      isTokenExpired: () => {
        const { expiresAt } = get();
        if (!expiresAt) return true;
        return new Date(expiresAt) <= new Date();
      },
    }),
    {
      name: 'streamspace-auth',
    }
  )
);
