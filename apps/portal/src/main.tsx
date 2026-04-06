import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { AuthProvider } from '@mrsmith/auth-client';
import { App } from './App';
import './styles/global.css';

function renderFatalError(message: string) {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <main
        style={{
          minHeight: '100vh',
          display: 'grid',
          placeItems: 'center',
          padding: '2rem',
        }}
      >
        <div
          style={{
            maxWidth: '42rem',
            border: '1px solid rgba(255, 104, 104, 0.35)',
            background: 'rgba(19, 10, 10, 0.92)',
            color: '#ff9090',
            padding: '1.5rem',
            fontFamily: '"Share Tech Mono", monospace',
            letterSpacing: '0.04em',
            lineHeight: 1.6,
          }}
        >
          {message}
        </div>
      </main>
    </StrictMode>,
  );
}

async function bootstrap() {
  const res = await fetch('/config');
  if (!res.ok) {
    throw new Error(`Portal auth bootstrap failed with status ${res.status}.`);
  }

  const config: { keycloakUrl: string; realm: string; clientId: string } = await res.json();
  if (!config.keycloakUrl || !config.realm || !config.clientId) {
    throw new Error('Portal auth bootstrap is missing Keycloak frontend configuration.');
  }

  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <AuthProvider
        keycloakUrl={config.keycloakUrl}
        realm={config.realm}
        clientId={config.clientId}
      >
        <App />
      </AuthProvider>
    </StrictMode>,
  );
}

bootstrap().catch((error: unknown) => {
  const message = error instanceof Error ? error.message : 'Portal auth bootstrap failed.';
  renderFatalError(message);
});
