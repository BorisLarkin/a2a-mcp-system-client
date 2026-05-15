import { useState, useEffect, createContext, useContext } from 'react';
import { api, setTokens, clearTokens } from '@/api/client';

interface User {
  id: string;
  username: string;
  role: 'admin' | 'operator' | 'viewer';
}

interface AuthContextType {
  user: User | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
  isAuthenticated: boolean;
  isLoading: boolean;
  setupRequired: boolean;
}

const AuthContext = createContext<AuthContextType>(null!);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [setupRequired, setSetupRequired] = useState(false);

  useEffect(() => {
    checkAuth();
  }, []);

  const checkAuth = async () => {
    // Сначала проверяем, нужен ли setup
    try {
      const status = await api<{ setup_required: boolean }>('/setup/status');
      if (status.setup_required) {
        setSetupRequired(true);
        setLoading(false);
        return;
      }
    } catch {
      // Если /setup/status недоступен (например, обычный режим), продолжаем
    }

    // Проверяем токен
    const token = localStorage.getItem('access_token');
    if (!token) {
      setLoading(false);
      return;
    }

    try {
      const data = await api<{ user: User }>('/auth/me');
      setUser(data.user);
    } catch {
      clearTokens();
    }
    setLoading(false);
  };

  const login = async (username: string, password: string) => {
    const data = await api<{ access_token: string; refresh_token: string; user: User }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
    setTokens(data.access_token, data.refresh_token);
    setUser(data.user);
  };

  const logout = () => {
    clearTokens();
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{ user, login, logout, isAuthenticated: !!user, isLoading: loading, setupRequired }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}