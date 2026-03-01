'use client';

import { create } from 'zustand';
import { getMe, clearToken, setToken, type User } from '@/lib/api';

// -- Auth Store ---------------------------------------------------

interface AuthState {
    /** The authenticated user, or null if not yet loaded / logged out. */
    user: User | null;
    /** True while the initial getMe() call is in-flight. */
    loading: boolean;
    /** Fetch the current user from the API. No-op if already loaded. */
    fetchUser: () => Promise<void>;
    /** Set user directly (e.g., from login response) and store the token. */
    loginUser: (token: string, user: User) => void;
    /** Clear user state and token, triggering a redirect to /login. */
    logout: () => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
    user: null,
    loading: true,

    fetchUser: async () => {
        // Skip if user is already loaded
        if (get().user) {
            set({ loading: false });
            return;
        }

        set({ loading: true });
        try {
            const user = await getMe();
            set({ user, loading: false });
        } catch {
            clearToken();
            set({ user: null, loading: false });
        }
    },

    loginUser: (token: string, user: User) => {
        setToken(token);
        set({ user, loading: false });
    },

    logout: () => {
        clearToken();
        set({ user: null, loading: false });
    },
}));
