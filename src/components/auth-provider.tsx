"use client";

import {
  type ReactNode,
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  type RefreshResponse,
  type TokenResponse,
  type User,
  login as loginRequest,
  logout as logoutRequest,
  refresh as refreshRequest,
  register as registerRequest,
  updateProfile as updateProfileRequest,
} from "@/lib/auth-api";

type Session = {
  accessToken: string;
  user: User;
  expiresAt: number;
};

type AuthContextValue = {
  user: User | null;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  updateProfile: (name: string) => Promise<void>;
  authorizedFetch: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

type JwtClaims = {
  sub?: string;
  email?: string;
  name?: string;
  exp?: number;
};

function decodeAccessToken(accessToken: string): JwtClaims {
  const payload = accessToken.split(".")[1];

  if (!payload) {
    throw new Error("Некорректный access token");
  }

  const normalizedPayload = payload.replace(/-/g, "+").replace(/_/g, "/");
  const bytes = Uint8Array.from(window.atob(normalizedPayload), (character) => character.charCodeAt(0));
  const decoded = new TextDecoder().decode(bytes);
  return JSON.parse(decoded) as JwtClaims;
}

function sessionFromToken(accessToken: string, user?: User): Session {
  const claims = decodeAccessToken(accessToken);

  if (!claims.sub || !claims.email || !claims.exp) {
    throw new Error("В access token отсутствуют необходимые данные пользователя");
  }

  return {
    accessToken,
    expiresAt: claims.exp * 1000,
    user: user ?? { id: claims.sub, email: claims.email, name: claims.name ?? "", createdAt: "" },
  };
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const sessionRef = useRef<Session | null>(null);
  const refreshRequestRef = useRef<Promise<RefreshResponse> | null>(null);

  const applySession = useCallback((accessToken: string, user?: User) => {
    const nextSession = sessionFromToken(accessToken, user);
    sessionRef.current = nextSession;
    setSession(nextSession);
  }, []);

  const clearSession = useCallback(() => {
    sessionRef.current = null;
    setSession(null);
  }, []);

  const renewAccessToken = useCallback(async () => {
    if (!refreshRequestRef.current) {
      refreshRequestRef.current = refreshRequest();
    }

    try {
      const response = await refreshRequestRef.current;
      applySession(response.accessToken);
      return response.accessToken;
    } catch (error) {
      clearSession();
      throw error;
    } finally {
      refreshRequestRef.current = null;
    }
  }, [applySession, clearSession]);

  useEffect(() => {
    void renewAccessToken().finally(() => setIsLoading(false));
  }, [renewAccessToken]);

  useEffect(() => {
    if (!session) {
      return;
    }

    const delay = Math.max(0, session.expiresAt - Date.now() - 60_000);
    const timer = window.setTimeout(() => {
      void renewAccessToken();
    }, delay);

    return () => window.clearTimeout(timer);
  }, [renewAccessToken, session]);

  const login = useCallback(async (email: string, password: string) => {
    const response: TokenResponse = await loginRequest({ email, password });
    applySession(response.accessToken, response.user);
  }, [applySession]);

  const register = useCallback(async (email: string, password: string) => {
    const response: TokenResponse = await registerRequest({ email, password });
    applySession(response.accessToken, response.user);
  }, [applySession]);

  const logout = useCallback(async () => {
	await logoutRequest();
	clearSession();
  }, [clearSession]);

  const updateProfile = useCallback(async (name: string) => {
    const currentSession = sessionRef.current;
    if (!currentSession) {
      throw new Error("Пользователь не авторизован");
    }
    const user = await updateProfileRequest(name, currentSession.accessToken);
    applySession(currentSession.accessToken, user);
  }, [applySession]);

  const authorizedFetch = useCallback(async (input: RequestInfo | URL, init: RequestInit = {}) => {
    const send = (accessToken: string) => fetch(input, {
      ...init,
      credentials: "include",
      headers: { ...init.headers, Authorization: `Bearer ${accessToken}` },
    });
    const currentSession = sessionRef.current;

    if (!currentSession) {
      throw new Error("Пользователь не авторизован");
    }

    const response = await send(currentSession.accessToken);
    if (response.status !== 401) {
      return response;
    }

    return send(await renewAccessToken());
  }, [renewAccessToken]);

  const value = useMemo<AuthContextValue>(() => ({
    user: session?.user ?? null,
    isLoading,
    login,
    register,
    logout,
    updateProfile,
    authorizedFetch,
  }), [authorizedFetch, isLoading, login, logout, register, session, updateProfile]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);

  if (!context) {
    throw new Error("useAuth must be used inside AuthProvider");
  }

  return context;
}
