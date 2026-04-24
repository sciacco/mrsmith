import { Button, Icon, Modal, SearchInput, Skeleton, useToast } from '@mrsmith/ui';
import { useDeferredValue, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useLLMModelMutations, useLLMModels } from '../api/queries';
import type { LLMModel } from '../api/types';
import { RequiredMark } from '../components/RequiredMark';
import { errorMessage } from '../lib/format';
import shared from './shared.module.css';

const scopePattern = /^[a-z][a-z0-9_]*$/;

const clockFormatter = new Intl.DateTimeFormat('it-IT', {
  hour: '2-digit',
  minute: '2-digit',
});

function formatClockTime(timestamp: number): string {
  return clockFormatter.format(new Date(timestamp));
}

export function ConfigurationLLMModelsPage() {
  const navigate = useNavigate();
  const models = useLLMModels();
  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search);
  const [editing, setEditing] = useState<LLMModel | null>(null);
  const [modalOpen, setModalOpen] = useState(false);

  const filteredModels = useMemo(() => {
    const q = deferredSearch.trim().toLowerCase();
    if (!q) return models.data ?? [];
    return (models.data ?? []).filter((item) =>
      `${item.scope} ${item.model}`.toLowerCase().includes(q),
    );
  }, [deferredSearch, models.data]);

  const hasSearch = search.trim().length > 0;
  const isEmpty = !models.isLoading && !models.error && filteredModels.length === 0;
  const lastUpdated = models.dataUpdatedAt ? formatClockTime(models.dataUpdatedAt) : null;

  return (
    <section className={shared.page}>
      <button type="button" className={shared.backLink} onClick={() => navigate('/manutenzioni/configurazione')}>
        <Icon name="chevron-left" size={16} />
        Torna alla configurazione
      </button>

      <div className={shared.header}>
        <div className={shared.titleBlock}>
          <h1 className={shared.pageTitle}>Modelli AI</h1>
          <p className={shared.pageSubtitle}>Modelli usati dalle automazioni delle manutenzioni.</p>
        </div>
        <div className={shared.headerActions}>
          <Button
            variant="secondary"
            onClick={() => models.refetch()}
            loading={models.isFetching && !models.isLoading}
            leftIcon={<Icon name="loader" size={16} />}
          >
            Aggiorna
          </Button>
          <Button
            onClick={() => {
              setEditing(null);
              setModalOpen(true);
            }}
            leftIcon={<Icon name="plus" size={16} />}
          >
            Nuovo modello
          </Button>
        </div>
      </div>

      <div className={shared.filterBar} style={{ gridTemplateColumns: 'minmax(240px, 1fr)' }}>
        <SearchInput
          value={search}
          onChange={setSearch}
          placeholder="Cerca per ambito o modello..."
        />
      </div>

      {models.isLoading ? (
        <div className={shared.panel}>
          <Skeleton rows={5} />
        </div>
      ) : models.error ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIconDanger}>
            <Icon name="triangle-alert" />
          </div>
          <h3>Modelli non disponibili</h3>
          <p>{errorMessage(models.error, 'Impossibile caricare i modelli.')}</p>
        </div>
      ) : isEmpty ? (
        <LLMModelsEmptyState
          hasSearch={hasSearch}
          searchValue={search}
          onClearSearch={() => setSearch('')}
          onCreate={() => {
            setEditing(null);
            setModalOpen(true);
          }}
        />
      ) : (
        <div className={shared.tableCard}>
          <div className={shared.tableScroll}>
            <table className={shared.table} style={{ minWidth: 720 }}>
              <thead>
                <tr>
                  <th>Ambito</th>
                  <th>Modello</th>
                  <th className={shared.actionsCell}>Azioni</th>
                </tr>
              </thead>
              <tbody>
                {filteredModels.map((item) => (
                  <tr key={item.scope}>
                    <td className={shared.mono}>{item.scope}</td>
                    <td className={`${shared.mono} ${shared.modelValue}`}>{item.model}</td>
                    <td className={shared.actionsCell}>
                      <div className={shared.inlineActions}>
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() => {
                            setEditing(item);
                            setModalOpen(true);
                          }}
                          leftIcon={<Icon name="pencil" size={14} />}
                        >
                          Modifica
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className={shared.tableFooter}>
            <span>
              {models.data?.length ?? 0} {(models.data?.length ?? 0) === 1 ? 'modello' : 'modelli'} totali
            </span>
            {lastUpdated ? <span>Aggiornato alle {lastUpdated}</span> : null}
          </div>
        </div>
      )}

      <LLMModelModal
        open={modalOpen}
        item={editing}
        onClose={() => setModalOpen(false)}
        onSaved={() => {
          setModalOpen(false);
          setEditing(null);
        }}
      />
    </section>
  );
}

function LLMModelsEmptyState({
  hasSearch,
  searchValue,
  onClearSearch,
  onCreate,
}: {
  hasSearch: boolean;
  searchValue: string;
  onClearSearch: () => void;
  onCreate: () => void;
}) {
  if (hasSearch) {
    return (
      <div className={shared.emptyCard}>
        <div className={shared.emptyIcon}>
          <Icon name="search" />
        </div>
        <h3>Nessun modello trovato</h3>
        <p>Nessun modello corrisponde a &ldquo;{searchValue}&rdquo;.</p>
        <div style={{ marginTop: '1rem' }}>
          <Button variant="secondary" onClick={onClearSearch}>
            Cancella ricerca
          </Button>
        </div>
      </div>
    );
  }
  return (
    <div className={shared.emptyCard}>
      <div className={shared.emptyIcon}>
        <Icon name="settings" />
      </div>
      <h3>Nessun modello configurato</h3>
      <p>Aggiungi il primo modello per renderlo disponibile alle automazioni.</p>
      <div style={{ marginTop: '1rem' }}>
        <Button onClick={onCreate} leftIcon={<Icon name="plus" size={16} />}>
          Nuovo modello
        </Button>
      </div>
    </div>
  );
}

type LLMFormState = {
  scope: string;
  model: string;
};

type LLMFormErrors = Partial<Record<keyof LLMFormState, string>>;

function LLMModelModal({
  open,
  item,
  onClose,
  onSaved,
}: {
  open: boolean;
  item: LLMModel | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const mutations = useLLMModelMutations();
  const toast = useToast();
  const [form, setForm] = useState<LLMFormState>(() => formFromModel(item));
  const [errors, setErrors] = useState<LLMFormErrors>({});
  const [submitted, setSubmitted] = useState(false);

  useEffect(() => {
    if (open) {
      setForm(formFromModel(item));
      setErrors({});
      setSubmitted(false);
    }
  }, [item, open]);

  function update<K extends keyof LLMFormState>(key: K, value: LLMFormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
    if (submitted) {
      setErrors((prev) => {
        if (!prev[key]) return prev;
        const next = { ...prev };
        delete next[key];
        return next;
      });
    }
  }

  function validate(): LLMFormErrors {
    const next: LLMFormErrors = {};
    const scope = form.scope.trim();
    const model = form.model.trim();
    if (!item && !scope) next.scope = "L'ambito è obbligatorio.";
    else if (!item && !scopePattern.test(scope)) {
      next.scope = 'Usa lettere minuscole, numeri e underscore.';
    }
    if (!model) next.model = 'Il modello è obbligatorio.';
    return next;
  }

  async function save() {
    setSubmitted(true);
    const nextErrors = validate();
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) return;

    const body = {
      scope: item?.scope ?? form.scope.trim(),
      model: form.model.trim(),
    };
    try {
      if (item) await mutations.update.mutateAsync(body);
      else await mutations.create.mutateAsync(body);
      toast.toast('Modello salvato.');
      onSaved();
    } catch (error) {
      toast.toast(errorMessage(error, 'Salvataggio non riuscito.'), 'error');
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={item ? 'Modifica modello' : 'Nuovo modello'}
      size="lg"
    >
      <div className={shared.formGrid}>
        <label className={shared.label}>
          <span className={shared.labelText}>
            Ambito<RequiredMark />
          </span>
          <input
            className={`${shared.field} ${errors.scope ? shared.fieldInvalid : ''}`}
            value={form.scope}
            disabled={Boolean(item)}
            autoFocus={!item}
            onChange={(event) => update('scope', event.target.value)}
          />
          {errors.scope ? (
            <span className={shared.fieldError}>{errors.scope}</span>
          ) : item ? (
            <span className={shared.fieldHelper}>L&apos;ambito non è modificabile dopo la creazione.</span>
          ) : (
            <span className={shared.fieldHelper}>
              Identificativo in minuscolo, es. <code>assistance_draft</code>.
            </span>
          )}
        </label>
        <label className={shared.label}>
          <span className={shared.labelText}>
            Modello<RequiredMark />
          </span>
          <input
            className={`${shared.field} ${errors.model ? shared.fieldInvalid : ''}`}
            value={form.model}
            autoFocus={Boolean(item)}
            onChange={(event) => update('model', event.target.value)}
          />
          {errors.model ? <span className={shared.fieldError}>{errors.model}</span> : null}
        </label>
      </div>
      <div className={shared.formActions} style={{ marginTop: '1rem' }}>
        <Button variant="secondary" onClick={onClose}>
          Annulla
        </Button>
        <Button onClick={save} loading={mutations.create.isPending || mutations.update.isPending}>
          Salva
        </Button>
      </div>
    </Modal>
  );
}

function formFromModel(item: LLMModel | null): LLMFormState {
  return {
    scope: item?.scope ?? '',
    model: item?.model ?? '',
  };
}
