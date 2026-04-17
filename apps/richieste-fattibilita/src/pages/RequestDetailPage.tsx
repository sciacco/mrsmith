import { Button, Icon, Modal, MultiSelect, SearchInput, SingleSelect, Skeleton, ToggleSwitch, Tooltip, useToast } from '@mrsmith/ui';
import { hasAnyRole } from '@mrsmith/auth-client';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useCreateFattibilita, useFornitori, useRichiestaFull, useTecnologie, useUpdateFattibilita, useUpdateRichiestaStato } from '../api/queries';
import type { Fattibilita, UpdateFattibilitaBody } from '../api/types';
import { StatusPill, statusTone } from '../components/StatusPill';
import { BUDGET_OPTIONS, FATTIBILITA_STATES, MANAGER_ROLES, RICHIESTA_STATES, budgetLabel, copyErrorMessage, fattibilitaStateLabel, formatCounts, formatCurrencyEuro, parsePositiveId, relativeTime, richiestaStateLabel } from '../lib/format';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import styles from './shared.module.css';

interface FattibilitaFormState {
  descrizione: string;
  contatto_fornitore: string;
  riferimento_fornitore: string;
  stato: string;
  annotazioni: string;
  esito_ricevuto_il: string;
  da_ordinare: boolean;
  profilo_fornitore: string;
  nrc: string;
  mrc: string;
  durata_mesi: string;
  aderenza_budget: string;
  copertura: boolean;
  giorni_rilascio: string;
}

function toFormState(item: Fattibilita): FattibilitaFormState {
  return {
    descrizione: item.descrizione ?? '',
    contatto_fornitore: item.contatto_fornitore ?? '',
    riferimento_fornitore: item.riferimento_fornitore ?? '',
    stato: item.stato,
    annotazioni: item.annotazioni ?? '',
    esito_ricevuto_il: item.esito_ricevuto_il ?? '',
    da_ordinare: item.da_ordinare,
    profilo_fornitore: item.profilo_fornitore ?? '',
    nrc: item.nrc == null ? '' : String(item.nrc),
    mrc: item.mrc == null ? '' : String(item.mrc),
    durata_mesi: item.durata_mesi == null ? '' : String(item.durata_mesi),
    aderenza_budget: String(item.aderenza_budget),
    copertura: item.copertura,
    giorni_rilascio: item.giorni_rilascio == null ? '' : String(item.giorni_rilascio),
  };
}

function formStatesEqual(a: FattibilitaFormState, b: FattibilitaFormState): boolean {
  return (
    a.descrizione === b.descrizione &&
    a.contatto_fornitore === b.contatto_fornitore &&
    a.riferimento_fornitore === b.riferimento_fornitore &&
    a.stato === b.stato &&
    a.annotazioni === b.annotazioni &&
    a.esito_ricevuto_il === b.esito_ricevuto_il &&
    a.da_ordinare === b.da_ordinare &&
    a.profilo_fornitore === b.profilo_fornitore &&
    a.nrc === b.nrc &&
    a.mrc === b.mrc &&
    a.durata_mesi === b.durata_mesi &&
    a.aderenza_budget === b.aderenza_budget &&
    a.copertura === b.copertura &&
    a.giorni_rilascio === b.giorni_rilascio
  );
}

// Transitions that cannot be undone from the UI — require explicit confirm.
const DESTRUCTIVE_RICHIESTA_TRANSITIONS: Record<string, string[]> = {
  // from any state → "annullata"
  '*': ['annullata'],
  // from completata → earlier states
  completata: ['nuova', 'in corso'],
};

function isDestructiveRichiestaTransition(from: string, to: string): boolean {
  if (from === to) return false;
  const any = DESTRUCTIVE_RICHIESTA_TRANSITIONS['*'] ?? [];
  if (any.includes(to)) return true;
  const specific = DESTRUCTIVE_RICHIESTA_TRANSITIONS[from] ?? [];
  return specific.includes(to);
}

export function RequestDetailPage() {
  const navigate = useNavigate();
  const params = useParams();
  const { user } = useOptionalAuth();
  const { toast } = useToast();
  const richiestaId = parsePositiveId(params.id);
  const canManage = hasAnyRole(user?.roles, MANAGER_ROLES);

  const richiesta = useRichiestaFull(richiestaId);
  const fornitori = useFornitori();
  const tecnologie = useTecnologie();
  const updateRichiestaStato = useUpdateRichiestaStato();
  const updateFattibilita = useUpdateFattibilita();
  const createFattibilita = useCreateFattibilita();

  const [selectedFattibilitaId, setSelectedFattibilitaId] = useState<number | null>(null);
  const [formState, setFormState] = useState<FattibilitaFormState | null>(null);
  const [showAddModal, setShowAddModal] = useState(false);
  const [selectedFornitori, setSelectedFornitori] = useState<number[]>([]);
  const [selectedTecnologie, setSelectedTecnologie] = useState<number[]>([]);
  const [pendingSelectionId, setPendingSelectionId] = useState<number | null>(null);
  const [pendingStato, setPendingStato] = useState<string | null>(null);
  const [showResetConfirm, setShowResetConfirm] = useState(false);
  const [listSearch, setListSearch] = useState('');

  const selectedFattibilita = useMemo(
    () => richiesta.data?.fattibilita.find((item) => item.id === selectedFattibilitaId) ?? null,
    [richiesta.data, selectedFattibilitaId],
  );

  const baselineFormState = useMemo(
    () => (selectedFattibilita ? toFormState(selectedFattibilita) : null),
    [selectedFattibilita],
  );

  const isDirty = Boolean(formState && baselineFormState && !formStatesEqual(formState, baselineFormState));

  useEffect(() => {
    if (!richiesta.data) return;
    if (richiesta.data.fattibilita.length === 0) {
      setSelectedFattibilitaId(null);
      setFormState(null);
      return;
    }
    const firstItem = richiesta.data.fattibilita[0];
    if (!selectedFattibilitaId || !richiesta.data.fattibilita.some((item) => item.id === selectedFattibilitaId)) {
      if (firstItem) setSelectedFattibilitaId(firstItem.id);
    }
  }, [richiesta.data, selectedFattibilitaId]);

  useEffect(() => {
    if (!selectedFattibilita) return;
    setFormState(toFormState(selectedFattibilita));
  }, [selectedFattibilita]);

  useEffect(() => {
    if (!isDirty) return;
    function onBeforeUnload(event: BeforeUnloadEvent) {
      event.preventDefault();
      event.returnValue = '';
    }
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [isDirty]);

  if (!canManage) {
    return (
      <section className={styles.forbiddenCard}>
        <div className={styles.emptyIconDanger}><Icon name="lock" /></div>
        <h3>Accesso riservato</h3>
        <p className={styles.muted}>Il dettaglio carrier è disponibile solo per il ruolo manager RDF.</p>
      </section>
    );
  }

  if (richiestaId === null) {
    return (
      <section className={styles.emptyCard}>
        <div className={styles.emptyIconDanger}><Icon name="triangle-alert" /></div>
        <h3>Identificativo non valido</h3>
        <p className={styles.muted}>Il dettaglio richiesto non può essere aperto con questo URL.</p>
      </section>
    );
  }

  function handleUpdateField<K extends keyof FattibilitaFormState>(key: K, value: FattibilitaFormState[K]) {
    setFormState((current) => (current ? { ...current, [key]: value } : current));
  }

  function buildPatchBody(state: FattibilitaFormState): UpdateFattibilitaBody {
    const payload: UpdateFattibilitaBody = {
      descrizione: state.descrizione,
      contatto_fornitore: state.contatto_fornitore,
      riferimento_fornitore: state.riferimento_fornitore,
      stato: state.stato,
      annotazioni: state.annotazioni,
      esito_ricevuto_il: state.esito_ricevuto_il,
      da_ordinare: state.da_ordinare,
      profilo_fornitore: state.profilo_fornitore,
      copertura: state.copertura,
    };
    if (state.nrc !== '') payload.nrc = Number(state.nrc);
    if (state.mrc !== '') payload.mrc = Number(state.mrc);
    if (state.durata_mesi !== '') payload.durata_mesi = Number(state.durata_mesi);
    if (state.aderenza_budget !== '') payload.aderenza_budget = Number(state.aderenza_budget);
    if (state.giorni_rilascio !== '') payload.giorni_rilascio = Number(state.giorni_rilascio);
    return payload;
  }

  const handleSaveRef = useRef<() => void>(() => {});
  const handleSave = useCallback(() => {
    if (!selectedFattibilita || !formState || !richiesta.data || !isDirty) return;
    updateFattibilita.mutate(
      {
        fattibilitaId: selectedFattibilita.id,
        richiestaId: richiestaId!,
        body: buildPatchBody(formState),
      },
      {
        onSuccess: () => toast('Fattibilità aggiornata'),
        onError: (error) => toast(copyErrorMessage(error, 'Aggiornamento non riuscito.'), 'error'),
      },
    );
  }, [selectedFattibilita, formState, richiesta.data, isDirty, richiestaId, updateFattibilita, toast]);
  handleSaveRef.current = handleSave;

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      const isSave = (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 's';
      if (!isSave) return;
      event.preventDefault();
      handleSaveRef.current();
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  const filteredFattibilita = useMemo(() => {
    const all = richiesta.data?.fattibilita ?? [];
    const q = listSearch.trim().toLowerCase();
    if (!q) return all;
    return all.filter(
      (item) =>
        item.fornitore_nome.toLowerCase().includes(q) ||
        item.tecnologia_nome.toLowerCase().includes(q) ||
        item.stato.toLowerCase().includes(q),
    );
  }, [richiesta.data, listSearch]);

  const existingPairs = useMemo(() => {
    const set = new Set<string>();
    richiesta.data?.fattibilita.forEach((item) => set.add(`${item.fornitore_id}:${item.tecnologia_id}`));
    return set;
  }, [richiesta.data]);

  const addPreview = useMemo(() => {
    const pairs = selectedFornitori.flatMap((fornitoreId) =>
      selectedTecnologie.map((tecnologiaId) => ({ fornitore_id: fornitoreId, tecnologia_id: tecnologiaId })),
    );
    const dedupedItems = pairs.filter((pair) => !existingPairs.has(`${pair.fornitore_id}:${pair.tecnologia_id}`));
    return { total: pairs.length, duplicates: pairs.length - dedupedItems.length, items: dedupedItems };
  }, [selectedFornitori, selectedTecnologie, existingPairs]);

  function handleAddFattibilita() {
    if (addPreview.items.length === 0) return;
    createFattibilita.mutate(
      { richiestaId: richiestaId!, body: { items: addPreview.items } },
      {
        onSuccess: () => {
          toast(addPreview.items.length === 1 ? 'Riga aggiunta' : `${addPreview.items.length} righe aggiunte`);
          setShowAddModal(false);
          setSelectedFornitori([]);
          setSelectedTecnologie([]);
        },
        onError: (error) => toast(copyErrorMessage(error, 'Inserimento non riuscito.'), 'error'),
      },
    );
  }

  function requestSelect(id: number) {
    if (id === selectedFattibilitaId) return;
    if (isDirty) {
      setPendingSelectionId(id);
      return;
    }
    setSelectedFattibilitaId(id);
  }

  function confirmSwitchSelection() {
    if (pendingSelectionId == null) return;
    setSelectedFattibilitaId(pendingSelectionId);
    setPendingSelectionId(null);
  }

  function requestRichiestaStatoChange(nextState: string) {
    if (!richiesta.data || richiesta.data.stato === nextState) return;
    if (isDestructiveRichiestaTransition(richiesta.data.stato, nextState)) {
      setPendingStato(nextState);
      return;
    }
    updateRichiestaStato.mutate(
      { richiestaId: richiestaId!, body: { stato: nextState } },
      {
        onSuccess: () => toast('Stato richiesta aggiornato'),
        onError: (error) => toast(copyErrorMessage(error, 'Aggiornamento stato non riuscito.'), 'error'),
      },
    );
  }

  function confirmRichiestaStatoChange() {
    if (!pendingStato) return;
    const next = pendingStato;
    setPendingStato(null);
    updateRichiestaStato.mutate(
      { richiestaId: richiestaId!, body: { stato: next } },
      {
        onSuccess: () => toast('Stato richiesta aggiornato'),
        onError: (error) => toast(copyErrorMessage(error, 'Aggiornamento stato non riuscito.'), 'error'),
      },
    );
  }

  function requestReset() {
    if (!isDirty) return;
    setShowResetConfirm(true);
  }

  function confirmReset() {
    if (selectedFattibilita) setFormState(toFormState(selectedFattibilita));
    setShowResetConfirm(false);
  }

  return (
    <section className={styles.page}>
      {richiesta.isLoading ? (
        <div className={styles.panel}>
          <Skeleton rows={8} />
        </div>
      ) : richiesta.error || !richiesta.data ? (
        <div className={styles.emptyCard}>
          <div className={styles.emptyIconDanger}><Icon name="triangle-alert" /></div>
          <h3>Dettaglio non disponibile</h3>
          <p className={styles.muted}>{copyErrorMessage(richiesta.error, 'Impossibile caricare la richiesta.')}</p>
        </div>
      ) : (
        <>
          <div className={styles.pageHeader}>
            <div>
              <button
                type="button"
                className={styles.breadcrumbLink}
                onClick={() => navigate('/richieste/gestione')}
              >
                ‹ Tutte le richieste
              </button>
              <h1 className={styles.pageTitle}>Dettaglio RDF Carrier</h1>
              <p className={styles.pageSubtitle}>
                Aggiorna i dati ricevuti dai carrier e gestisci lo stato della richiesta.
              </p>
            </div>
            <div className={styles.headerActions}>
              <Button className={styles.flatBtn} onClick={() => navigate(`/richieste/${richiestaId}/view`)}>
                Visualizza RDF
              </Button>
            </div>
          </div>

          <div className={styles.heroCard}>
            <div className={styles.heroLead}>
              <div className={styles.heroTitleStack}>
                <span className={styles.summaryCode}>{richiesta.data.codice_deal || `RDF #${richiesta.data.id}`}</span>
                <div className={styles.contextTitle}>
                  {richiesta.data.company_name ?? 'Cliente non disponibile'}
                </div>
                <span className={styles.heroDeal}>{richiesta.data.deal_name ?? 'Deal non disponibile'}</span>
              </div>
              <div
                className={styles.statusRow}
                role="radiogroup"
                aria-label="Stato richiesta"
              >
                {RICHIESTA_STATES.map((state) => {
                  const active = richiesta.data?.stato === state;
                  return (
                    <button
                      key={state}
                      type="button"
                      role="radio"
                      aria-checked={active}
                      className={`${styles.statusChip} ${active ? styles.statusChipActive : ''}`}
                      onClick={() => requestRichiestaStatoChange(state)}
                    >
                      {richiestaStateLabel(state)}
                    </button>
                  );
                })}
              </div>
            </div>

            <div className={styles.heroInlineMeta}>
              <span><strong>Indirizzo</strong>{richiesta.data.indirizzo}</span>
              <span><strong>Richiedente</strong>{richiesta.data.created_by ?? 'Non disponibile'}</span>
              <span><strong>Avanzamento</strong>{formatCounts(richiesta.data.counts)}</span>
            </div>
          </div>

          <div className={styles.gridTwo}>
            <div className={styles.listCard}>
              <div className={styles.panelHeader}>
                <div>
                  <h2 className={styles.panelTitle}>Carriers</h2>
                  <p className={styles.small}>
                    {richiesta.data.fattibilita.length === 0
                      ? 'Aggiungi almeno un fornitore per iniziare.'
                      : `${richiesta.data.fattibilita.length} righe · ${richiesta.data.counts.completata} completate`}
                  </p>
                </div>
                <Tooltip content="Aggiungi Fornitori">
                  <button
                    type="button"
                    className={styles.iconBtn}
                    onClick={() => setShowAddModal(true)}
                    aria-label="Aggiungi Fornitori"
                  >
                    <Icon name="plus" />
                  </button>
                </Tooltip>
              </div>

              {richiesta.data.fattibilita.length > 3 && (
                <div className={styles.listToolbar}>
                  <SearchInput
                    value={listSearch}
                    onChange={setListSearch}
                    placeholder="Filtra per fornitore, tecnologia o stato…"
                  />
                </div>
              )}

              <div
                className={styles.listWrap}
                role="radiogroup"
                aria-label="Seleziona fattibilità"
                onKeyDown={(event) => {
                  if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
                  if (filteredFattibilita.length === 0) return;
                  event.preventDefault();
                  const currentIndex = filteredFattibilita.findIndex((item) => item.id === selectedFattibilitaId);
                  const delta = event.key === 'ArrowDown' ? 1 : -1;
                  const nextIndex = currentIndex === -1
                    ? (delta === 1 ? 0 : filteredFattibilita.length - 1)
                    : (currentIndex + delta + filteredFattibilita.length) % filteredFattibilita.length;
                  const target = filteredFattibilita[nextIndex];
                  if (target) requestSelect(target.id);
                }}
              >
                {richiesta.data.fattibilita.length === 0 ? (
                  <div className={styles.emptyCard}>
                    <div className={styles.emptyIcon}><Icon name="package" /></div>
                    <h3>Nessuna riga presente</h3>
                    <p className={styles.muted}>Aggiungi almeno una combinazione fornitore-tecnologia per iniziare.</p>
                  </div>
                ) : filteredFattibilita.length === 0 ? (
                  <div className={styles.emptyCard}>
                    <p className={styles.muted}>Nessun risultato per «{listSearch}».</p>
                  </div>
                ) : (
                  filteredFattibilita.map((item) => {
                    const selected = selectedFattibilitaId === item.id;
                    const rowIsDirty = selected && isDirty;
                    const needsAttention = item.stato === 'bozza' || item.stato === 'sollecitata';
                    const hasNrc = item.nrc != null && item.nrc > 0;
                    const hasMrc = item.mrc != null && item.mrc > 0;
                    const hasBudget = item.aderenza_budget > 0;
                    const relative = relativeTime(item.esito_ricevuto_il);
                    const metaParts: string[] = [];
                    if (hasNrc) metaParts.push(`NRC ${formatCurrencyEuro(item.nrc)}`);
                    if (hasMrc) metaParts.push(`MRC ${formatCurrencyEuro(item.mrc)}/mese`);
                    if (hasBudget) metaParts.push(`Budget ${budgetLabel(item.aderenza_budget)}`);
                    if (relative) metaParts.push(`Esito ${relative}`);
                    return (
                      <button
                        key={item.id}
                        type="button"
                        role="radio"
                        aria-checked={selected}
                        className={`${styles.listItem} ${selected ? styles.listItemSelected : ''} ${needsAttention ? styles.listItemAttention : ''}`}
                        onClick={() => requestSelect(item.id)}
                        aria-label={rowIsDirty ? `${item.fornitore_nome} — modifiche non salvate` : undefined}
                      >
                        <div className={styles.listItemTop}>
                          <div>
                            <div className={styles.listHeading}>{item.fornitore_nome}</div>
                            <p className={styles.small}>{item.tecnologia_nome}</p>
                          </div>
                          <div className={styles.listItemStatus}>
                            <StatusPill tone={statusTone(item.stato)}>{fattibilitaStateLabel(item.stato)}</StatusPill>
                            {rowIsDirty && (
                              <span className={styles.dirtyBadge} aria-label="Modifiche non salvate">Non salvata</span>
                            )}
                          </div>
                        </div>
                        <div className={styles.listItemMeta}>
                          {metaParts.length > 0 ? (
                            metaParts.map((part) => <span key={part}>{part}</span>)
                          ) : (
                            <span className={styles.muted}>Dati non ancora ricevuti</span>
                          )}
                        </div>
                      </button>
                    );
                  })
                )}
              </div>
            </div>

            <div className={styles.formCard}>
              <div className={styles.panelHeader}>
                <div>
                  <h2 className={styles.panelTitle}>Scheda di fattibilità</h2>
                  <p className={styles.small}>Aggiorna i dati ricevuti dal fornitore e salva le note operative.</p>
                </div>
              </div>

              {!selectedFattibilita || !formState ? (
                <div className={styles.emptyCard}>
                  <div className={styles.emptyIcon}><Icon name="pencil" /></div>
                  <h3>Seleziona una riga</h3>
                  <p className={styles.muted}>Scegli una fattibilità dalla lista per aprire il modulo di modifica.</p>
                </div>
              ) : (
                <div className={styles.formGrid}>
                  <section className={styles.formSection} aria-labelledby="rdf-section-esito">
                    <header className={styles.formSectionHeader}>
                      <span id="rdf-section-esito" className={styles.formSectionTitle}>Esito</span>
                    </header>

                    <div className={styles.toggleRow}>
                      <ToggleSwitch
                        id="copertura"
                        checked={formState.copertura}
                        onChange={(checked) => handleUpdateField('copertura', checked)}
                        label="Copertura presente"
                      />
                      <ToggleSwitch
                        id="da_ordinare"
                        checked={formState.da_ordinare}
                        onChange={(checked) => handleUpdateField('da_ordinare', checked)}
                        label="Da ordinare"
                      />
                    </div>

                    <div className={styles.fieldRowPair}>
                      <div className={styles.fieldRow}>
                        <label htmlFor="fattibilita-stato">Stato</label>
                        <SingleSelect
                          options={FATTIBILITA_STATES.map((value) => ({ value, label: fattibilitaStateLabel(value) }))}
                          selected={formState.stato}
                          onChange={(value) => handleUpdateField('stato', value ?? formState.stato)}
                        />
                      </div>
                      <div className={styles.fieldRow}>
                        <label htmlFor="esito_ricevuto_il">Esito ricevuto il</label>
                        <input
                          id="esito_ricevuto_il"
                          type="date"
                          className={styles.dateInput}
                          value={formState.esito_ricevuto_il}
                          onChange={(event) => handleUpdateField('esito_ricevuto_il', event.target.value)}
                        />
                      </div>
                    </div>
                  </section>

                  <section className={styles.formSection} aria-labelledby="rdf-section-offerta">
                    <header className={styles.formSectionHeader}>
                      <span id="rdf-section-offerta" className={styles.formSectionTitle}>Offerta commerciale</span>
                    </header>

                    <div className={styles.fieldRow}>
                      <label htmlFor="profilo_fornitore">Profilo fornitore</label>
                      <input
                        id="profilo_fornitore"
                        className={styles.fieldInput}
                        value={formState.profilo_fornitore}
                        onChange={(event) => handleUpdateField('profilo_fornitore', event.target.value)}
                      />
                    </div>
                    <div className={styles.commercialGrid}>
                      <div className={styles.fieldGroup}>
                        <label htmlFor="nrc">NRC (€)</label>
                        <input
                          id="nrc"
                          type="number"
                          step="0.01"
                          className={styles.numberInput}
                          value={formState.nrc}
                          onChange={(event) => handleUpdateField('nrc', event.target.value)}
                        />
                      </div>
                      <div className={styles.fieldGroup}>
                        <label htmlFor="mrc">MRC (€/mese)</label>
                        <input
                          id="mrc"
                          type="number"
                          step="0.01"
                          className={styles.numberInput}
                          value={formState.mrc}
                          onChange={(event) => handleUpdateField('mrc', event.target.value)}
                        />
                      </div>
                      <div className={styles.fieldGroup}>
                        <label htmlFor="durata_mesi">Durata (mesi)</label>
                        <input
                          id="durata_mesi"
                          type="number"
                          className={styles.numberInput}
                          value={formState.durata_mesi}
                          onChange={(event) => handleUpdateField('durata_mesi', event.target.value)}
                        />
                      </div>
                      <div className={styles.fieldGroup}>
                        <label htmlFor="giorni_rilascio">Giorni rilascio</label>
                        <input
                          id="giorni_rilascio"
                          type="number"
                          className={styles.numberInput}
                          value={formState.giorni_rilascio}
                          onChange={(event) => handleUpdateField('giorni_rilascio', event.target.value)}
                        />
                      </div>
                    </div>
                    <div className={styles.fieldRow}>
                      <label id="aderenza-budget-label">Aderenza budget</label>
                      <div
                        className={styles.segmented}
                        role="radiogroup"
                        aria-labelledby="aderenza-budget-label"
                      >
                        {BUDGET_OPTIONS.map((option) => {
                          const active = Number(formState.aderenza_budget) === option.value;
                          return (
                            <button
                              key={option.value}
                              type="button"
                              role="radio"
                              aria-checked={active}
                              className={`${styles.segmentedOption} ${active ? styles.segmentedOptionActive : ''}`}
                              onClick={() => handleUpdateField('aderenza_budget', String(option.value))}
                            >
                              {option.label}
                            </button>
                          );
                        })}
                      </div>
                    </div>
                  </section>

                  <section className={styles.formSection} aria-labelledby="rdf-section-contatto">
                    <header className={styles.formSectionHeader}>
                      <span id="rdf-section-contatto" className={styles.formSectionTitle}>Contatto carrier</span>
                    </header>

                    <div className={styles.fieldRow}>
                      <label htmlFor="contatto_fornitore">Contatto fornitore</label>
                      <input
                        id="contatto_fornitore"
                        className={styles.fieldInput}
                        value={formState.contatto_fornitore}
                        onChange={(event) => handleUpdateField('contatto_fornitore', event.target.value)}
                      />
                    </div>
                    <div className={styles.fieldRow}>
                      <label htmlFor="riferimento_fornitore">Riferimento fornitore</label>
                      <input
                        id="riferimento_fornitore"
                        className={styles.fieldInput}
                        value={formState.riferimento_fornitore}
                        onChange={(event) => handleUpdateField('riferimento_fornitore', event.target.value)}
                      />
                    </div>
                  </section>

                  <section className={styles.formSection} aria-labelledby="rdf-section-note">
                    <header className={styles.formSectionHeader}>
                      <span id="rdf-section-note" className={styles.formSectionTitle}>Note</span>
                    </header>

                    <div className={styles.fieldGroup}>
                      <label htmlFor="descrizione_ff">Descrizione</label>
                      <textarea
                        id="descrizione_ff"
                        className={styles.textArea}
                        value={formState.descrizione}
                        onChange={(event) => handleUpdateField('descrizione', event.target.value)}
                      />
                    </div>

                    <div className={styles.fieldGroup}>
                      <label htmlFor="annotazioni">Annotazioni</label>
                      <textarea
                        id="annotazioni"
                        className={styles.textArea}
                        value={formState.annotazioni}
                        onChange={(event) => handleUpdateField('annotazioni', event.target.value)}
                      />
                    </div>
                  </section>

                  <div className={styles.stickyActionBar}>
                    <div className={styles.stickyActionBarHint} aria-live="polite">
                      {isDirty
                        ? 'Modifiche non salvate · premi ⌘S per salvare'
                        : 'Tutte le modifiche salvate'}
                    </div>
                    <div className={styles.stickyActionBarButtons}>
                      <Button variant="secondary" className={styles.flatBtn} onClick={requestReset} disabled={!isDirty}>
                        Ripristina
                      </Button>
                      <Button className={styles.flatBtn} onClick={handleSave} loading={updateFattibilita.isPending} disabled={!isDirty}>
                        Salva modifiche
                      </Button>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>
        </>
      )}

      <Modal open={showAddModal} onClose={() => setShowAddModal(false)} title="Aggiungi righe di fattibilità">
        <div className={styles.modalBody}>
          <div className={styles.fieldGroup}>
            <label htmlFor="modal-fornitori">Fornitori</label>
            <MultiSelect
              options={(fornitori.data ?? []).map((item) => ({ value: item.id, label: item.nome }))}
              selected={selectedFornitori}
              onChange={setSelectedFornitori}
              placeholder={fornitori.isLoading ? 'Caricamento fornitori...' : 'Seleziona fornitori'}
            />
          </div>
          <div className={styles.fieldGroup}>
            <label htmlFor="modal-tecnologie">Tecnologie</label>
            <MultiSelect
              options={(tecnologie.data ?? []).map((item) => ({ value: item.id, label: item.nome }))}
              selected={selectedTecnologie}
              onChange={setSelectedTecnologie}
              placeholder={tecnologie.isLoading ? 'Caricamento tecnologie...' : 'Seleziona tecnologie'}
            />
          </div>
          {addPreview.total > 0 && (
            <p className={styles.small} aria-live="polite">
              {addPreview.duplicates > 0
                ? `Verranno create ${addPreview.items.length} righe · ${addPreview.duplicates} duplicati verranno ignorati`
                : `Verranno create ${addPreview.items.length} righe`}
            </p>
          )}
          <div className={styles.actionsRow}>
            <Button className={styles.flatBtn} onClick={handleAddFattibilita} loading={createFattibilita.isPending} disabled={addPreview.items.length === 0}>
              Crea combinazioni
            </Button>
            <Button variant="secondary" className={styles.flatBtn} onClick={() => setShowAddModal(false)}>
              Annulla
            </Button>
          </div>
        </div>
      </Modal>

      <Modal
        open={pendingSelectionId !== null}
        onClose={() => setPendingSelectionId(null)}
        title="Scartare le modifiche?"
        size="sm"
      >
        <div className={styles.modalBody}>
          <p className={styles.muted}>
            Le modifiche alla riga selezionata non sono state salvate. Continuando andranno perse.
          </p>
          <div className={styles.actionsRow}>
            <Button variant="danger" className={styles.flatBtn} onClick={confirmSwitchSelection}>
              Scarta e cambia riga
            </Button>
            <Button variant="secondary" className={styles.flatBtn} onClick={() => setPendingSelectionId(null)}>
              Continua a modificare
            </Button>
          </div>
        </div>
      </Modal>

      <Modal
        open={pendingStato !== null}
        onClose={() => setPendingStato(null)}
        title={pendingStato === 'annullata' ? 'Annullare la richiesta?' : 'Confermare il cambio di stato?'}
        size="sm"
      >
        <div className={styles.modalBody}>
          <p className={styles.muted}>
            {pendingStato === 'annullata'
              ? 'La richiesta verrà annullata. I carrier non riceveranno ulteriori aggiornamenti.'
              : `La richiesta tornerà allo stato "${pendingStato}". Confermi?`}
          </p>
          <div className={styles.actionsRow}>
            <Button variant="danger" className={styles.flatBtn} onClick={confirmRichiestaStatoChange} loading={updateRichiestaStato.isPending}>
              {pendingStato === 'annullata' ? 'Annulla richiesta' : 'Conferma'}
            </Button>
            <Button variant="secondary" className={styles.flatBtn} onClick={() => setPendingStato(null)}>
              Chiudi
            </Button>
          </div>
        </div>
      </Modal>

      <Modal
        open={showResetConfirm}
        onClose={() => setShowResetConfirm(false)}
        title="Ripristinare i valori originali?"
        size="sm"
      >
        <div className={styles.modalBody}>
          <p className={styles.muted}>Tutte le modifiche non salvate di questa riga verranno perse.</p>
          <div className={styles.actionsRow}>
            <Button variant="danger" className={styles.flatBtn} onClick={confirmReset}>Ripristina</Button>
            <Button variant="secondary" className={styles.flatBtn} onClick={() => setShowResetConfirm(false)}>Continua a modificare</Button>
          </div>
        </div>
      </Modal>
    </section>
  );
}
