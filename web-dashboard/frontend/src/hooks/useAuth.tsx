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
}

const AuthContext = createContext<AuthContextType>(null!);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = localStorage.getItem('access_token');
    if (token) {
      api<{ user: User }>('/auth/me')
        .then(d => setUser(d.user))
        .catch(() => {
          // Пробуем обновить токен
          const refresh = localStorage.getItem('refresh_token');
          if (refresh) {
            api<{ access_token: string; refresh_token: string; user: User }>('/auth/refresh', {
              method: 'POST',
              body: JSON.stringify({ refresh_token: refresh }),
            })
              .then(d => {
                setTokens(d.access_token, d.refresh_token);
                setUser(d.user);
              })
              .catch(() => clearTokens());
          } else {
            clearTokens();
          }
        })
        .finally(() => setLoading(false));
    } else {
      setLoading(false);
    }
  }, []);

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

  if (loading) return <div className="flex items-center justify-center min-h-screen">Загрузка...</div>;

  return (
    <AuthContext.Provider value={{ user, login, logout, isAuthenticated: !!user }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}