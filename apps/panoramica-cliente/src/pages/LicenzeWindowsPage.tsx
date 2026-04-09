import { ApiError } from '@mrsmith/api-client';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { useWindowsLicenses } from '../api/queries';
import { ServiceUnavailable } from '../components/shared/ServiceUnavailable';
import s from './shared.module.css';

export function LicenzeWindowsPage() {
  const licensesQ = useWindowsLicenses();

  if (licensesQ.error && (licensesQ.error as ApiError).status === 503) {
    return <ServiceUnavailable service="Grappa" />;
  }

  const data = licensesQ.data ?? [];

  return (
    <div className={s.page}>
      <h2 style={{ marginBottom: 'var(--space-2)' }}>Licenze Windows</h2>
      <p style={{ color: 'var(--color-text-muted)', marginBottom: 'var(--space-6)', fontSize: '0.875rem' }}>
        Conteggio giornaliero licenze Windows attive (ultimi 14 giorni)
      </p>

      {licensesQ.isLoading && <div className={s.loading}>Caricamento...</div>}

      {data.length > 0 && (
        <div style={{ width: '100%', height: 400 }}>
          <ResponsiveContainer>
            <BarChart data={[...data].reverse()} margin={{ top: 10, right: 30, left: 10, bottom: 10 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
              <XAxis
                dataKey="x"
                tick={{ fontSize: 11 }}
                tickFormatter={(v: string) => v.slice(5)}
              />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip
                contentStyle={{
                  background: 'var(--color-bg-elevated)',
                  border: '1px solid var(--color-border)',
                  borderRadius: 8,
                  fontSize: '0.8125rem',
                }}
              />
              <Bar dataKey="y" name="Licenze" fill="var(--color-accent)" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      {!licensesQ.isLoading && data.length === 0 && !licensesQ.error && (
        <div className={s.empty}>Nessun dato disponibile.</div>
      )}
    </div>
  );
}
