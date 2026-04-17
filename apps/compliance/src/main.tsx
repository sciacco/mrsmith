import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { AuthProvider } from '@mrsmith/auth-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ApiError, isLocalAuthPreflightUnauthorized } from '@mrsmith/api-client';
import { BrowserRouter } from 'react-router-dom';
import { ToastProvider } from '@mrsmith/ui';
import { App } from './App';
import './styles/global.css';

const routerBasename =
  import.meta.env.BASE_URL === '/'
    ? '/'
    : import.meta.env.BASE_URL.replace(/\/$/, '');

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000,
      refetchOnWindowFocus: false,
      retry: (failureCount, error) => {
        if (error instanceof ApiError) {
          if (error.status === 403) return false;
          if (error.status === 401 && !isLocalAuthPreflightUnauthorized(error)) return false;
        }
        return failureCount < 3;
      },
    },
  },
});

async function bootstrap() {
  const res = await fetch('/config');
  if (!res.ok) {
    throw new Error(`Compliance auth bootstrap failed with status ${res.status}.`);
  }

  const config: { keycloakUrl: string; realm: string; clientId: string } = await res.json();
  if (!config.keycloakUrl || !config.realm || !config.clientId) {
    throw new Error('Compliance auth bootstrap is missing Keycloak frontend configuration.');
  }

  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <AuthProvider
        keycloakUrl={config.keycloakUrl}
        realm={config.realm}
        clientId={config.clientId}
      >
        <QueryClientProvider client={queryClient}>
          <BrowserRouter basename={routerBasename}>
            <ToastProvider>
              <App />
            </ToastProvider>
          </BrowserRouter>
        </QueryClientProvider>
      </AuthProvider>
    </StrictMode>,
  );
}

function renderFatalError(message: string) {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <main
        style={{
          minHeight: '100vh',
          display: 'grid',
          placeItems: 'center',
          padding: '2rem',
          fontFamily: 'system-ui, sans-serif',
        }}
      >
        <div
          style={{
            maxWidth: '40rem',
            border: '1px solid rgba(220, 53, 69, 0.18)',
            background: '#fff',
            color: '#7b1f2d',
            padding: '1.5rem',
            borderRadius: '0.75rem',
            lineHeight: 1.6,
          }}
        >
          {message}
        </div>
      </main>
    </StrictMode>,
  );
}

bootstrap().catch((error: unknown) => {
  const message = error instanceof Error ? error.message : 'Compliance auth bootstrap failed.';
  renderFatalError(message);
});
