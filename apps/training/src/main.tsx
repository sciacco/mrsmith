import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { AuthProvider } from '@mrsmith/auth-client';
import { ApiError, isLocalAuthPreflightUnauthorized } from '@mrsmith/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import { ToastProvider } from '@mrsmith/ui';
import { App } from './App';
import './styles/global.css';

document.documentElement.dataset.theme = 'clean';

const routerBasename =
  import.meta.env.BASE_URL === '/'
    ? '/'
    : import.meta.env.BASE_URL.replace(/\/$/, '');

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 2 * 60 * 1000,
      refetchOnWindowFocus: false,
      retry: (failureCount, error) => {
        if (error instanceof ApiError) {
          if (error.status === 403) return false;
          if (error.status === 401 && !isLocalAuthPreflightUnauthorized(error)) return false;
        }
        return failureCount < 2;
      },
    },
  },
});

interface BootstrapConfig {
  keycloakUrl: string;
  realm: string;
  clientId: string;
}

async function bootstrap() {
  const res = await fetch('/config');
  if (!res.ok) {
    throw new Error(`Bootstrap Formazione non riuscito (${res.status}).`);
  }
  const config: BootstrapConfig = await res.json();
  if (!config.keycloakUrl || !config.realm || !config.clientId) {
    throw new Error('Configurazione di accesso non disponibile.');
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
      <main className="fatalPage">
        <div className="fatalCard">{message}</div>
      </main>
    </StrictMode>,
  );
}

bootstrap().catch((error: unknown) => {
  const message = error instanceof Error ? error.message : 'Formazione non disponibile.';
  renderFatalError(message);
});
