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

      // Set authentication from login response
      setAuth: (loginResponse: LoginResponse) =>
        set({
          user: loginResponse.user,
          token: loginResponse.token,
          expiresAt: loginResponse.expiresAt,
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
