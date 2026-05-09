import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { AuthProvider } from '@mrsmith/auth-client';
import { ApiError, isLocalAuthPreflightUnauthorized } from '@mrsmith/api-client';
import { ToastProvider } from '@mrsmith/ui';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
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
      staleTime: 5 * 60 * 1000,
      refetchOnWindowFocus: false,
      retry: (failureCount, error) => {
        if (error instanceof ApiError) {
          if (error.status === 403) return false;
          if (error.status === 401 && !isLocalAuthPreflightUnauthorized(error)) return false;
          if (error.status === 503) return false;
        }
        return failureCount < 3;
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
    throw new Error(`Grappa DCIM bootstrap failed with status ${res.status}.`);
  }

  const config: BootstrapConfig = await res.json();
  if (!config.keycloakUrl || !config.realm || !config.clientId) {
    throw new Error('Grappa DCIM bootstrap is missing Keycloak frontend configuration.');
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

function renderFatalError() {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <main className="fatalShell">
        <div className="fatalCard">
          <strong>Accesso non disponibile</strong>
          Non e stato possibile aprire Grappa DCIM in questo momento. Ricarica la pagina o riapri
          l&apos;app dal portale.
        </div>
      </main>
    </StrictMode>,
  );
}

bootstrap().catch((error: unknown) => {
  console.error('Grappa DCIM bootstrap failed.', error);
  renderFatalError();
});
