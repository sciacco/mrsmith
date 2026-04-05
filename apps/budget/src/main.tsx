import { StrictMode, type ReactNode } from 'react';
import { createRoot } from 'react-dom/client';
import { AuthProvider } from '@mrsmith/auth-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import { ToastProvider } from '@mrsmith/ui';
import { App } from './App';
import './styles/global.css';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000,
      refetchOnWindowFocus: false,
    },
  },
});

async function bootstrap() {
  const res = await fetch('/config');
  const config: { keycloakUrl: string; realm: string; clientId: string } = await res.json();

  const hasAuth = Boolean(config.keycloakUrl && config.realm && config.clientId);

  function AuthWrapper({ children }: { children: ReactNode }) {
    if (!hasAuth) return <>{children}</>;
    return (
      <AuthProvider
        keycloakUrl={config.keycloakUrl}
        realm={config.realm}
        clientId={config.clientId}
      >
        {children}
      </AuthProvider>
    );
  }

  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <AuthWrapper>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter>
            <ToastProvider>
              <App />
            </ToastProvider>
          </BrowserRouter>
        </QueryClientProvider>
      </AuthWrapper>
    </StrictMode>,
  );
}

bootstrap();
