export function ServiceUnavailable({ service }: { service: string }) {
  return (
    <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--color-text-secondary)' }}>
      <h2>Servizio non disponibile</h2>
      <p>La connessione a {service} non è configurata. Questa sezione non è al momento disponibile.</p>
    </div>
  );
}
