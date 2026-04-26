import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { AuthProvider } from '@mrsmith/auth-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ApiError, isLocalAuthPreflightUnauthorized } from '@mrsmith/api-client';
import { BrowserRouter } from 'react-router-dom';
import { ToastProvider } from '@mrsmith/ui';
import { App } from './App';
import './styles/global.css';

document.documentElement.dataset.theme = 'clean';

const routerBasename = import.meta.env.BASE_URL === '/' ? '/' : import.meta.env.BASE_URL.replace(/\/$/, '');

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 3 * 60 * 1000,
      refetchOnWindowFocus: false,
      retry: (failureCount, error) => {
        if (error instanceof ApiError) {
          if (error.status === 403 || error.status === 503) return false;
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
  if (!res.ok) throw new Error(`Fornitori bootstrap failed with status ${res.status}.`);
  const config: BootstrapConfig = await res.json();

  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <AuthProvider keycloakUrl={config.keycloakUrl} realm={config.realm} clientId={config.clientId}>
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
    <main className="fatal">
      <section>
        <strong>Accesso non disponibile</strong>
        <p>Non e stato possibile aprire Fornitori in questo momento. Ricarica la pagina o riapri l&apos;app dal portale.</p>
      </section>
    </main>,
  );
}

bootstrap().catch((error: unknown) => {
  console.error('Fornitori bootstrap failed.', error);
  renderFatalError();
});
