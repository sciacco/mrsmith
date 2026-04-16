import { Button, Icon, Modal, MultiSelect, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import { hasAnyRole } from '@mrsmith/auth-client';
import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useCreateFattibilita, useFornitori, useRichiestaFull, useTecnologie, useUpdateFattibilita, useUpdateRichiestaStato } from '../api/queries';
import type { Fattibilita, UpdateFattibilitaBody } from '../api/types';
import { StatusPill, statusTone } from '../components/StatusPill';
import { BUDGET_OPTIONS, FATTIBILITA_STATES, MANAGER_ROLES, RICHIESTA_STATES, budgetLabel, copyErrorMessage, formatCounts, formatDate, formatDateTime, parsePositiveId } from '../lib/format';
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

  const selectedFattibilita = useMemo(
    () => richiesta.data?.fattibilita.find((item) => item.id === selectedFattibilitaId) ?? null,
    [richiesta.data, selectedFattibilitaId],
  );

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

  function handleSave() {
    if (!selectedFattibilita || !formState || !richiesta.data) return;
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
  }

  function handleAddFattibilita() {
    const items = selectedFornitori.flatMap((fornitoreId) =>
      selectedTecnologie.map((tecnologiaId) => ({ fornitore_id: fornitoreId, tecnologia_id: tecnologiaId })),
    );
    if (items.length === 0) return;
    createFattibilita.mutate(
      { richiestaId: richiestaId!, body: { items } },
      {
        onSuccess: () => {
          toast('Righe aggiunte');
          setShowAddModal(false);
          setSelectedFornitori([]);
          setSelectedTecnologie([]);
        },
        onError: (error) => toast(copyErrorMessage(error, 'Inserimento non riuscito.'), 'error'),
      },
    );
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
              <h1 className={styles.pageTitle}>Dettaglio RDF Carrier</h1>
              <p className={styles.pageSubtitle}>
                Gestisci lo stato della richiesta, aggiungi le combinazioni fornitore-tecnologia e aggiorna i dati ricevuti dai carrier.
              </p>
            </div>
            <div className={styles.headerActions}>
              <Button variant="secondary" onClick={() => navigate('/richieste/gestione')}>
                Torna alla gestione
              </Button>
              <Button onClick={() => navigate(`/richieste/${richiestaId}/view`)}>
                Visualizza RDF
              </Button>
            </div>
          </div>

          <div className={styles.heroCard}>
            <div className={styles.heroTop}>
              <div>
                <div className={styles.summaryCode}>{richiesta.data.codice_deal || `RDF #${richiesta.data.id}`}</div>
                <div className={styles.contextTitle}>
                  {richiesta.data.company_name ?? 'Cliente non disponibile'}
                </div>
                <p className={styles.pageSubtitle}>{richiesta.data.deal_name ?? 'Deal non disponibile'}</p>
              </div>
              <StatusPill tone={statusTone(richiesta.data.stato)} aria-label={`Stato ${richiesta.data.stato}`}>
                {richiesta.data.stato}
              </StatusPill>
            </div>

            <div className={styles.heroMeta}>
              <div>
                <p className={styles.small}>Richiesta #{richiesta.data.id}</p>
                <p>{richiesta.data.indirizzo}</p>
              </div>
              <div>
                <p className={styles.small}>Richiedente</p>
                <p>{richiesta.data.created_by ?? 'Non disponibile'}</p>
              </div>
              <div>
                <p className={styles.small}>Contatori</p>
                <p>{formatCounts(richiesta.data.counts)}</p>
              </div>
            </div>

            <div
              className={styles.statusRow}
              role="radiogroup"
              aria-label="Aggiorna stato richiesta"
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
                    onClick={() =>
                      updateRichiestaStato.mutate(
                        { richiestaId: richiestaId!, body: { stato: state } },
                        {
                          onSuccess: () => toast('Stato richiesta aggiornato'),
                          onError: (error) => toast(copyErrorMessage(error, 'Aggiornamento stato non riuscito.'), 'error'),
                        },
                      )
                    }
                  >
                    {state}
                  </button>
                );
              })}
            </div>
          </div>

          <div className={styles.gridTwo}>
            <div className={styles.listCard}>
              <div className={styles.panelHeader}>
                <div>
                  <h2 className={styles.panelTitle}>Righe fattibilità</h2>
                  <p className={styles.small}>Seleziona una riga per modificarla oppure aggiungine di nuove in batch.</p>
                </div>
                <Button onClick={() => setShowAddModal(true)}>Aggiungi righe</Button>
              </div>

              <div className={styles.listWrap} role="radiogroup" aria-label="Seleziona fattibilità">
                {richiesta.data.fattibilita.length === 0 ? (
                  <div className={styles.emptyCard}>
                    <div className={styles.emptyIcon}><Icon name="package" /></div>
                    <h3>Nessuna riga presente</h3>
                    <p className={styles.muted}>Aggiungi almeno una combinazione fornitore-tecnologia per iniziare.</p>
                  </div>
                ) : (
                  richiesta.data.fattibilita.map((item) => {
                    const selected = selectedFattibilitaId === item.id;
                    return (
                      <button
                        key={item.id}
                        type="button"
                        role="radio"
                        aria-checked={selected}
                        className={`${styles.listItem} ${selected ? styles.listItemSelected : ''}`}
                        onClick={() => setSelectedFattibilitaId(item.id)}
                      >
                        <div className={styles.listItemTop}>
                          <div>
                            <div className={styles.listHeading}>{item.fornitore_nome}</div>
                            <p className={styles.small}>{item.tecnologia_nome}</p>
                          </div>
                          <StatusPill tone={statusTone(item.stato)}>{item.stato}</StatusPill>
                        </div>
                        <div className={styles.listItemBottom}>
                          <span className={styles.small}>Budget: {budgetLabel(item.aderenza_budget)}</span>
                          <span className={styles.small}>Esito: {formatDate(item.esito_ricevuto_il)}</span>
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
                  <h2 className={styles.panelTitle}>Editor fattibilità</h2>
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
                  <div className={styles.fieldGroup}>
                    <label>Fornitore / Tecnologia</label>
                    <div className={styles.metaCard}>
                      <div className={styles.metaCardValue}>{selectedFattibilita.fornitore_nome}</div>
                      <p className={styles.small}>{selectedFattibilita.tecnologia_nome}</p>
                      <p className={styles.small}>Richiesta inviata il {formatDate(selectedFattibilita.data_richiesta)}</p>
                    </div>
                  </div>

                  <div className={styles.fieldGroup}>
                    <label htmlFor="fattibilita-stato">Stato</label>
                    <SingleSelect
                      options={FATTIBILITA_STATES.map((value) => ({ value, label: value }))}
                      selected={formState.stato}
                      onChange={(value) => handleUpdateField('stato', value ?? formState.stato)}
                    />
                  </div>

                  <div className={styles.fieldGroup}>
                    <label htmlFor="descrizione_ff">Descrizione</label>
                    <textarea
                      id="descrizione_ff"
                      className={styles.textArea}
                      value={formState.descrizione}
                      onChange={(event) => handleUpdateField('descrizione', event.target.value)}
                    />
                  </div>

                  <div className={styles.formColumns}>
                    <div className={styles.fieldGroup}>
                      <label htmlFor="contatto_fornitore">Contatto fornitore</label>
                      <input
                        id="contatto_fornitore"
                        className={styles.fieldInput}
                        value={formState.contatto_fornitore}
                        onChange={(event) => handleUpdateField('contatto_fornitore', event.target.value)}
                      />
                    </div>
                    <div className={styles.fieldGroup}>
                      <label htmlFor="riferimento_fornitore">Riferimento fornitore</label>
                      <input
                        id="riferimento_fornitore"
                        className={styles.fieldInput}
                        value={formState.riferimento_fornitore}
                        onChange={(event) => handleUpdateField('riferimento_fornitore', event.target.value)}
                      />
                    </div>
                  </div>

                  <div className={styles.formColumns}>
                    <div className={styles.fieldGroup}>
                      <label htmlFor="esito_ricevuto_il">Esito ricevuto il</label>
                      <input
                        id="esito_ricevuto_il"
                        type="date"
                        className={styles.dateInput}
                        value={formState.esito_ricevuto_il}
                        onChange={(event) => handleUpdateField('esito_ricevuto_il', event.target.value)}
                      />
                    </div>
                    <div className={styles.fieldGroup}>
                      <label htmlFor="profilo_fornitore">Profilo fornitore</label>
                      <input
                        id="profilo_fornitore"
                        className={styles.fieldInput}
                        value={formState.profilo_fornitore}
                        onChange={(event) => handleUpdateField('profilo_fornitore', event.target.value)}
                      />
                    </div>
                  </div>

                  <div className={styles.formColumns}>
                    <div className={styles.fieldGroup}>
                      <label htmlFor="nrc">NRC</label>
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
                      <label htmlFor="mrc">MRC</label>
                      <input
                        id="mrc"
                        type="number"
                        step="0.01"
                        className={styles.numberInput}
                        value={formState.mrc}
                        onChange={(event) => handleUpdateField('mrc', event.target.value)}
                      />
                    </div>
                  </div>

                  <div className={styles.formColumns}>
                    <div className={styles.fieldGroup}>
                      <label htmlFor="durata_mesi">Durata mesi</label>
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

                  <div className={styles.fieldGroup}>
                    <label htmlFor="aderenza-budget">Aderenza budget</label>
                    <SingleSelect
                      options={BUDGET_OPTIONS.map((item) => ({ value: item.value, label: item.label }))}
                      selected={Number(formState.aderenza_budget)}
                      onChange={(value) => handleUpdateField('aderenza_budget', String(value ?? 0))}
                    />
                  </div>

                  <div className={styles.toggleRow}>
                    <label className={styles.checkChip}>
                      <input
                        type="checkbox"
                        checked={formState.copertura}
                        onChange={(event) => handleUpdateField('copertura', event.target.checked)}
                      />
                      Copertura presente
                    </label>
                    <label className={styles.checkChip}>
                      <input
                        type="checkbox"
                        checked={formState.da_ordinare}
                        onChange={(event) => handleUpdateField('da_ordinare', event.target.checked)}
                      />
                      Da ordinare
                    </label>
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

                  <div className={styles.actionsRow}>
                    <Button onClick={handleSave} loading={updateFattibilita.isPending}>
                      Salva modifiche
                    </Button>
                    <Button variant="secondary" onClick={() => setFormState(toFormState(selectedFattibilita))}>
                      Ripristina
                    </Button>
                  </div>

                  <p className={styles.small}>Ultimo aggiornamento richiesta: {formatDateTime(richiesta.data.updated_at)}</p>
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
          <div className={styles.actionsRow}>
            <Button onClick={handleAddFattibilita} loading={createFattibilita.isPending} disabled={selectedFornitori.length === 0 || selectedTecnologie.length === 0}>
              Crea combinazioni
            </Button>
            <Button variant="secondary" onClick={() => setShowAddModal(false)}>
              Annulla
            </Button>
          </div>
        </div>
      </Modal>
    </section>
  );
}
