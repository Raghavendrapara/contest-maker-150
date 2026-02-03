import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios';
import type { ApiError } from '@/types';

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api';

// Create axios instance
export const api = axios.create({
    baseURL: API_BASE_URL,
    headers: {
        'Content-Type': 'application/json',
    },
});

// Token management
const TOKEN_KEY = 'contest_maker_tokens';

interface StoredTokens {
    accessToken: string;
    refreshToken: string;
    expiresAt: string;
}

export const getStoredTokens = (): StoredTokens | null => {
    const stored = localStorage.getItem(TOKEN_KEY);
    if (!stored) return null;
    try {
        return JSON.parse(stored);
    } catch {
        return null;
    }
};

export const setStoredTokens = (tokens: StoredTokens) => {
    localStorage.setItem(TOKEN_KEY, JSON.stringify(tokens));
};

export const clearStoredTokens = () => {
    localStorage.removeItem(TOKEN_KEY);
};

// Request interceptor - add auth token
api.interceptors.request.use(
    (config: InternalAxiosRequestConfig) => {
        const tokens = getStoredTokens();
        if (tokens?.accessToken) {
            config.headers.Authorization = `Bearer ${tokens.accessToken}`;
        }
        return config;
    },
    (error) => Promise.reject(error)
);

// Response interceptor - handle token refresh
let isRefreshing = false;
let refreshSubscribers: ((token: string) => void)[] = [];

const subscribeTokenRefresh = (callback: (token: string) => void) => {
    refreshSubscribers.push(callback);
};

const onTokenRefreshed = (token: string) => {
    refreshSubscribers.forEach((callback) => callback(token));
    refreshSubscribers = [];
};

api.interceptors.response.use(
    (response) => response,
    async (error: AxiosError<ApiError>) => {
        const originalRequest = error.config;

        if (error.response?.status === 401 && originalRequest) {
            const tokens = getStoredTokens();

            if (!tokens?.refreshToken) {
                clearStoredTokens();
                window.location.href = '/login';
                return Promise.reject(error);
            }

            if (isRefreshing) {
                return new Promise((resolve) => {
                    subscribeTokenRefresh((token) => {
                        originalRequest.headers.Authorization = `Bearer ${token}`;
                        resolve(api(originalRequest));
                    });
                });
            }

            isRefreshing = true;

            try {
                const response = await axios.post(`${API_BASE_URL}/auth/refresh`, {
                    refresh_token: tokens.refreshToken,
                });

                const newTokens = response.data.tokens;
                setStoredTokens({
                    accessToken: newTokens.access_token,
                    refreshToken: newTokens.refresh_token,
                    expiresAt: newTokens.expires_at,
                });

                onTokenRefreshed(newTokens.access_token);
                originalRequest.headers.Authorization = `Bearer ${newTokens.access_token}`;

                return api(originalRequest);
            } catch (refreshError) {
                clearStoredTokens();
                window.location.href = '/login';
                return Promise.reject(refreshError);
            } finally {
                isRefreshing = false;
            }
        }

        return Promise.reject(error);
    }
);

// API functions
export const authApi = {
    signup: async (data: { email: string; username: string; password: string }) => {
        const response = await api.post('/auth/signup', data);
        return response.data;
    },

    login: async (data: { email: string; password: string }) => {
        const response = await api.post('/auth/login', data);
        return response.data;
    },

    refresh: async (refreshToken: string) => {
        const response = await api.post('/auth/refresh', { refresh_token: refreshToken });
        return response.data;
    },
};

export const userApi = {
    getCurrentUser: async () => {
        const response = await api.get('/users/me');
        return response.data;
    },

    getProgress: async () => {
        const response = await api.get('/users/me/progress');
        return response.data;
    },
};

export const problemApi = {
    getAll: async () => {
        const response = await api.get('/problems');
        return response.data;
    },

    getStats: async () => {
        const response = await api.get('/problems/stats');
        return response.data;
    },

    getById: async (id: string) => {
        const response = await api.get(`/problems/${id}`);
        return response.data;
    },
};

export const contestApi = {
    create: async (data: { problem_count: number; duration_minutes: number }) => {
        const response = await api.post('/contests', data);
        return response.data;
    },

    getAll: async () => {
        const response = await api.get('/contests');
        return response.data;
    },

    getActive: async () => {
        const response = await api.get('/contests/active');
        return response.data;
    },

    getById: async (id: string) => {
        const response = await api.get(`/contests/${id}`);
        return response.data;
    },

    markProblemComplete: async (contestId: string, problemId: string, isCompleted: boolean) => {
        const response = await api.patch(`/contests/${contestId}/problems/${problemId}`, {
            is_completed: isCompleted,
        });
        return response.data;
    },

    complete: async (id: string) => {
        const response = await api.post(`/contests/${id}/complete`);
        return response.data;
    },

    abandon: async (id: string) => {
        const response = await api.post(`/contests/${id}/abandon`);
        return response.data;
    },
};
