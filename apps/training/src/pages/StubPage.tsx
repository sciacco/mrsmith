interface StubPageProps {
  title: string;
  description?: string;
}

export function StubPage({ title, description }: StubPageProps) {
  return (
    <main style={{ padding: '2rem', maxWidth: '64rem', margin: '0 auto' }}>
      <h1 style={{ marginBottom: '0.5rem', fontSize: '1.5rem' }}>{title}</h1>
      <p style={{ color: 'var(--color-text-muted)' }}>
        {description ?? 'Sezione in arrivo. Stiamo riorganizzando il workspace formazione.'}
      </p>
    </main>
  );
}
