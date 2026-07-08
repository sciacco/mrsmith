import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, Skeleton } from '@mrsmith/ui';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useApiClient } from '../../api/client';
import type { MACardIRLItem, MAIRLSeedReport, MAIRLStatus } from '../../api/types';
import styles from './CompanyPanels.module.css';

const IRL_STATUS_CYCLE: Record<MAIRLStatus, MAIRLStatus> = {
  aperta: 'chiesta',
  chiesta: 'risposta',
  risposta: 'na',
  na: 'aperta',
};

const IRL_SOURCE_LABELS: Record<string, string> = {
  flag: 'flag',
  brief: 'brief',
  thesis: 'tesi',
  template: 'template',
  analyst: 'analista',
};

function errorLabel(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 404) return 'IRL non disponibile per questa card.';
    if (error.status === 403) return 'Non hai accesso a Binocolo.';
    if (error.status === 503) return 'Servizio non configurato in questo ambiente.';
    return `Richiesta non riuscita (${error.status}).`;
  }
  return 'Richiesta non riuscita.';
}

function EmptyState({ title, text }: { title: string; text: string }) {
  return (
    <div className={styles.emptyState}>
      <span className={styles.emptyStateIcon} aria-hidden="true">
        <Icon name="clipboard-check" size={28} />
      </span>
      <h2>{title}</h2>
      <p>{text}</p>
    </div>
  );
}

export function IRLPanel({
  initiativeId,
  companyKey,
  companyName,
}: {
  initiativeId: string;
  companyKey: string;
  companyName: string;
}) {
  const api = useApiClient();
  const base = `/binocolo/v1/ma/initiatives/${initiativeId}/cards/${encodeURIComponent(companyKey)}/irl`;
  const [seedReport, setSeedReport] = useState<MAIRLSeedReport | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editText, setEditText] = useState('');
  const [newQuestion, setNewQuestion] = useState('');
  const [newCategory, setNewCategory] = useState('');

  const query = useQuery({
    queryKey: ['ma-irl', initiativeId, companyKey],
    enabled: Boolean(initiativeId && companyKey),
    queryFn: () => api.get<{ items: MACardIRLItem[] }>(base),
  });
  const refresh = () => void query.refetch();

  const seed = useMutation({
    mutationFn: () => api.post<MAIRLSeedReport>(`${base}/seed`, {}),
    onSuccess: (report) => {
      setSeedReport(report);
      refresh();
    },
  });
  const addItem = useMutation({
    mutationFn: () => api.post<MACardIRLItem>(`${base}/items`, { category: newCategory.trim(), question: newQuestion.trim() }),
    onSuccess: () => {
      setNewQuestion('');
      refresh();
    },
  });
  const patchItem = useMutation({
    mutationFn: ({ itemId, patch }: { itemId: string; patch: Partial<Pick<MACardIRLItem, 'category' | 'question' | 'status'>> }) =>
      api.patch<MACardIRLItem>(`${base}/items/${itemId}`, patch),
    onSuccess: refresh,
  });
  const deleteItem = useMutation({
    mutationFn: (itemId: string) => api.delete<{ deleted: boolean }>(`${base}/items/${itemId}`),
    onSuccess: refresh,
  });
  const exportIRL = useMutation({
    mutationFn: () => api.postBlob(`${base}/export`),
    onSuccess: (blob) => {
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = `irl-${companyName.replace(/[^\w\s-]/g, '').trim().replace(/\s+/g, '-').toLowerCase() || companyKey}.xlsx`;
      anchor.click();
      URL.revokeObjectURL(url);
    },
  });

  if (query.isLoading) return <Skeleton rows={5} />;
  if (query.isError) {
    return (
      <div className={styles.statePanel} role="alert">
        <Icon name="triangle-alert" size={22} />
        <p>{errorLabel(query.error)}</p>
      </div>
    );
  }

  const items = query.data?.items ?? [];
  const categories: string[] = [];
  const grouped = new Map<string, MACardIRLItem[]>();
  for (const item of items) {
    const category = item.category || 'generale';
    if (!grouped.has(category)) {
      grouped.set(category, []);
      categories.push(category);
    }
    grouped.get(category)?.push(item);
  }

  const startEdit = (item: MACardIRLItem) => {
    setEditingId(item.id);
    setEditText(item.question);
  };
  const commitEdit = (item: MACardIRLItem) => {
    const question = editText.trim();
    setEditingId(null);
    if (question && question !== item.question) {
      patchItem.mutate({ itemId: item.id, patch: { question } });
    }
  };

  return (
    <div>
      <div className={styles.irlToolbar}>
        <Button onClick={() => seed.mutate()} loading={seed.isPending}>
          Semina dalle fonti
        </Button>
        <Button variant="secondary" onClick={() => exportIRL.mutate()} loading={exportIRL.isPending} disabled={items.length === 0}>
          Export XLSX
        </Button>
        {seedReport ? (
          <span className={styles.irlSeedNote}>
            {seedReport.inserted} voci nuove su {seedReport.proposed} proposte
            {seedReport.inserted < seedReport.proposed ? ' (le altre erano già in lista)' : ''}
          </span>
        ) : null}
        {seed.isError ? <span className={styles.irlSeedNote}>Seed non riuscito: riprova.</span> : null}
      </div>

      {items.length === 0 ? (
        <EmptyState
          title="IRL vuota"
          text="Semina dalle fonti disponibili (flag, brief, lettura di tesi, template della famiglia) o aggiungi le voci manualmente."
        />
      ) : (
        categories.map((category) => (
          <div key={category} className={styles.irlGroup}>
            <h4 className={styles.irlGroupTitle}>{category}</h4>
            {(grouped.get(category) ?? []).map((item) => (
              <div key={item.id} className={styles.irlRow}>
                <button
                  type="button"
                  className={`${styles.irlStatusChip} ${
                    item.status === 'chiesta' ? styles.irlStatusChiesta : item.status === 'risposta' ? styles.irlStatusRisposta : ''
                  }`}
                  title="Cambia stato"
                  onClick={() => patchItem.mutate({ itemId: item.id, patch: { status: IRL_STATUS_CYCLE[item.status] } })}
                >
                  {item.status}
                </button>
                {editingId === item.id ? (
                  <input
                    className={styles.irlEditInput}
                    value={editText}
                    autoFocus
                    onChange={(event) => setEditText(event.target.value)}
                    onBlur={() => commitEdit(item)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') commitEdit(item);
                      if (event.key === 'Escape') setEditingId(null);
                    }}
                  />
                ) : (
                  <span
                    className={`${styles.irlQuestion} ${item.status === 'na' ? styles.irlQuestionNa : ''}`}
                    role="button"
                    tabIndex={0}
                    onClick={() => startEdit(item)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault();
                        startEdit(item);
                      }
                    }}
                  >
                    {item.question}
                  </span>
                )}
                <span className={styles.irlSource}>{IRL_SOURCE_LABELS[item.source] ?? item.source}</span>
                <button type="button" className={styles.irlDelete} title="Elimina voce" onClick={() => deleteItem.mutate(item.id)}>
                  <Icon name="x-circle" size={16} />
                </button>
              </div>
            ))}
          </div>
        ))
      )}

      <form
        className={styles.irlAddForm}
        onSubmit={(event) => {
          event.preventDefault();
          if (newQuestion.trim()) addItem.mutate();
        }}
      >
        <input className={styles.irlAddCategory} value={newCategory} placeholder="Categoria" onChange={(event) => setNewCategory(event.target.value)} />
        <input className={styles.irlAddInput} value={newQuestion} placeholder="Nuova richiesta…" onChange={(event) => setNewQuestion(event.target.value)} />
        <Button type="submit" variant="secondary" loading={addItem.isPending} disabled={!newQuestion.trim()}>
          Aggiungi
        </Button>
      </form>
    </div>
  );
}
