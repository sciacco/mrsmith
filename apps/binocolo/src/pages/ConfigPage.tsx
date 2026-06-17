import { Button, Icon, Skeleton, useToast } from '@mrsmith/ui';
import { useCallback, useEffect, useState } from 'react';
import { useApiClient } from '../api/client';
import type { MAParameter, MAParametersResponse } from '../api/types';
import styles from './ConfigPage.module.css';

export function ConfigPage() {
  const api = useApiClient();
  const { toast } = useToast();
  const [params, setParams] = useState<MAParameter[]>([]);
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [savingKey, setSavingKey] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.get<MAParametersResponse>('/binocolo/v1/ma/parameters');
      setParams(data.items);
      setDrafts(Object.fromEntries(data.items.map((item) => [item.key, item.value])));
    } catch {
      setError('Impossibile caricare i parametri di configurazione.');
    } finally {
      setLoading(false);
    }
  }, [api]);

  useEffect(() => {
    void load();
  }, [load]);

  async function save(param: MAParameter) {
    const value = (drafts[param.key] ?? '').trim();
    setSavingKey(param.key);
    setError(null);
    try {
      await api.put<void>('/binocolo/v1/ma/parameters', { key: param.key, value });
      toast('Parametro aggiornato.', 'success');
      await load();
    } catch {
      toast('Salvataggio non riuscito: verifica il valore (numero ≥ 0).', 'error');
    } finally {
      setSavingKey(null);
    }
  }

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <span className={styles.eyebrow}>Binocolo</span>
          <h1>Configurazione</h1>
          <p>Leve di costo e valutazione usate dalla ricerca Target M&amp;A. Le modifiche valgono per tutte le ricerche.</p>
        </div>
        <Button variant="secondary" onClick={load} loading={loading} leftIcon={<Icon name="refresh-cw" />}>
          Aggiorna
        </Button>
      </header>

      {error ? (
        <div className={styles.errorPanel} role="alert">
          <Icon name="triangle-alert" size={18} />
          <span>{error}</span>
        </div>
      ) : null}

      {loading && params.length === 0 ? (
        <div className={styles.card}>
          <Skeleton rows={6} />
        </div>
      ) : (
        <div className={styles.card}>
          <ul className={styles.list}>
            {params.map((param) => {
              const draft = drafts[param.key] ?? '';
              const dirty = draft.trim() !== param.value.trim();
              return (
                <li key={param.key} className={styles.row}>
                  <div className={styles.rowInfo}>
                    <span className={styles.rowLabel}>{param.label}</span>
                    {param.description ? <span className={styles.rowDesc}>{param.description}</span> : null}
                    <code className={styles.rowKey}>{param.key}</code>
                  </div>
                  <div className={styles.rowEdit}>
                    <div className={styles.inputWrap}>
                      {param.valueType === 'money' ? <span className={styles.affix}>€</span> : null}
                      <input
                        type="number"
                        min={0}
                        step={param.valueType === 'money' ? 0.01 : 1}
                        value={draft}
                        onChange={(event) => setDrafts((current) => ({ ...current, [param.key]: event.target.value }))}
                      />
                      {param.valueType === 'percent' ? <span className={styles.affix}>%</span> : null}
                    </div>
                    <Button
                      size="sm"
                      onClick={() => save(param)}
                      loading={savingKey === param.key}
                      disabled={!dirty || draft.trim() === '' || !(Number.isFinite(Number(draft)) && Number(draft) >= 0)}
                    >
                      Salva
                    </Button>
                  </div>
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </main>
  );
}
