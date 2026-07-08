import { Button, Icon, Modal, MultiSelect, Skeleton, useToast } from '@mrsmith/ui';
import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type FormEvent } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useApiClient } from '../../api/client';
import type {
  MAAtecoCandidate,
  MAEstimate,
  MAInitiativeListResponse,
  MAInitiativeSummary,
  MAProvinceCatalogItem,
  MAProvinceCatalogResponse,
  MASessionDetail,
  MAStrategyConcept,
  MAStrategySpec,
  MAThesis,
} from '../../api/types';
import {
  errorLabel,
  estimateCountLabel,
  estimateUsesLowerBound,
  expandedEstimatePreview,
  normalizeSearchLimit,
  numberFormat,
  optionalNumber,
  provinceLabel,
  sectorBlocker,
  summarizeTerritory,
  territoryIsWide,
} from './helpers';
import styles from './Ricerche.module.css';

const minPromptLength = 24;

const thesisLabels: Record<MAThesis | 'unknown', string> = {
  generico: 'Generico',
  successione: 'Successione',
  crescita: 'Crescita',
  consolidamento: 'Consolidamento',
  tuck_in: 'Competenze',
  unknown: 'Non definita',
};

type BusyState = 'create' | 'estimate' | 'execute' | null;
type InitiativeMode = 'auto' | 'manual' | 'existing';

export function NuovaRicercaPage() {
  const api = useApiClient();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const requestedInitiativeId = searchParams.get('iniziativa')?.trim() ?? '';
  const { toast } = useToast();
  const [catalog, setCatalog] = useState<MAProvinceCatalogItem[]>([]);
  const [detail, setDetail] = useState<MASessionDetail | null>(null);
  const [strategy, setStrategy] = useState<MAStrategySpec | null>(null);
  const [prompt, setPrompt] = useState('');
  const [showRequestEditor, setShowRequestEditor] = useState(true);
  const [estimateFresh, setEstimateFresh] = useState(false);
  const [provinceModalOpen, setProvinceModalOpen] = useState(false);
  const [atecoOpen, setAtecoOpen] = useState(false);
  const [busy, setBusy] = useState<BusyState>(null);
  const [error, setError] = useState<string | null>(null);
  const [initiatives, setInitiatives] = useState<MAInitiativeSummary[]>([]);
  const [initiativesLoading, setInitiativesLoading] = useState(false);
  const [initiativeMode, setInitiativeMode] = useState<InitiativeMode>('auto');
  const [initiativeId, setInitiativeId] = useState('');
  const [manualInitiativeTitle, setManualInitiativeTitle] = useState('');
  const [initiativeError, setInitiativeError] = useState<string | null>(null);
  const appliedInitiativeParamRef = useRef(false);
  const initiativeSelectionTouchedRef = useRef(false);

  const loadInitiatives = useCallback(async () => {
    setInitiativesLoading(true);
    try {
      const data = await api.get<MAInitiativeListResponse>('/binocolo/v1/ma/initiatives');
      setInitiatives(data.items);
      if (!appliedInitiativeParamRef.current && requestedInitiativeId) {
        appliedInitiativeParamRef.current = true;
        const requested = data.items.find(
          (item) => item.id === requestedInitiativeId && !item.archivedAt && !item.deletedAt && !item.purgedAt,
        );
        if (requested && !initiativeSelectionTouchedRef.current) {
          setInitiativeMode('existing');
          setInitiativeId(requested.id);
          setInitiativeError(null);
        }
      }
    } catch (err) {
      setError(errorLabel(err));
    } finally {
      setInitiativesLoading(false);
    }
  }, [api, requestedInitiativeId]);

  useEffect(() => {
    let active = true;
    api
      .get<MAProvinceCatalogResponse>('/binocolo/v1/ma/catalog/provinces')
      .then((data) => {
        if (active) setCatalog(data.items);
      })
      .catch((err) => {
        if (active) setError(errorLabel(err));
      });
    return () => {
      active = false;
    };
  }, [api]);

  useEffect(() => {
    void loadInitiatives();
  }, [loadInitiatives]);

  useEffect(() => {
    const sessionId = detail?.session.id;
    if (!sessionId || detail?.session.status !== 'estimating') return;
    const handle = setInterval(() => {
      api
        .get<MASessionDetail>(`/binocolo/v1/ma/sessions/${sessionId}?targets=none`)
        .then((data) => {
          setDetail(data);
          if (data.strategy?.strategy) setStrategy(data.strategy.strategy);
          if (data.session.status !== 'estimating') {
            setEstimateFresh(true);
            setBusy(null);
          }
        })
        .catch((err) => {
          setError(errorLabel(err));
          setBusy(null);
        });
    }, 2500);
    return () => clearInterval(handle);
  }, [api, detail?.session.id, detail?.session.status]);

  const activeInitiatives = useMemo(
    () => initiatives.filter((item) => !item.archivedAt && !item.deletedAt && !item.purgedAt),
    [initiatives],
  );
  const blocker = strategy ? sectorBlocker(strategy) : null;
  const preview = estimateFresh ? expandedEstimatePreview(detail?.estimates ?? []) : null;
  const estimating = busy === 'estimate' || detail?.session.status === 'estimating';
  const canEstimate = Boolean(strategy) && !blocker && !estimating && busy !== 'create' && busy !== 'execute';
  const wideTerritory = strategy ? territoryIsWide(strategy.provinces, catalog) : false;

  useEffect(() => {
    if (initiativeMode !== 'existing') return;
    if (activeInitiatives.length === 0) {
      setInitiativeId('');
      setInitiativeMode('auto');
      return;
    }
    if (!activeInitiatives.some((item) => item.id === initiativeId)) {
      setInitiativeId(activeInitiatives[0]?.id ?? '');
    }
  }, [activeInitiatives, initiativeId, initiativeMode]);

  const updateStrategy = useCallback((patch: Partial<MAStrategySpec>) => {
    setStrategy((current) => (current ? { ...current, ...patch } : current));
    setEstimateFresh(false);
  }, []);

  async function createSession(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalized = prompt.trim();
    if (normalized.length < minPromptLength) return;

    const initiativePayload: { initiativeId?: string; newInitiativeTitle?: string } = {};
    setInitiativeError(null);
    if (initiativeMode === 'manual') {
      const title = manualInitiativeTitle.trim();
      if (!title) {
        setInitiativeError('Inserire un titolo iniziativa.');
        return;
      }
      initiativePayload.newInitiativeTitle = title;
    }
    if (initiativeMode === 'existing') {
      if (activeInitiatives.length === 0) {
        setInitiativeError('Creare una nuova iniziativa o usare il titolo automatico.');
        return;
      }
      if (!initiativeId) {
        setInitiativeError("Selezionare un'iniziativa attiva.");
        return;
      }
      initiativePayload.initiativeId = initiativeId;
    }

    setBusy('create');
    setError(null);
    try {
      const data = await api.post<MASessionDetail>('/binocolo/v1/ma/sessions?targets=none', {
        prompt: normalized,
        gatedFlow: true,
        ...initiativePayload,
      });
      if (!data.strategy?.strategy) {
        throw new Error('Perimetro non restituito dal server.');
      }
      setDetail(data);
      setStrategy(data.strategy.strategy);
      setEstimateFresh(false);
      setShowRequestEditor(false);
    } catch (err) {
      setError(errorLabel(err));
    } finally {
      setBusy(null);
    }
  }

  async function estimateSession() {
    if (!detail?.session.id || !strategy) return;
    setBusy('estimate');
    setError(null);
    try {
      const data = await api.post<MASessionDetail>(`/binocolo/v1/ma/sessions/${detail.session.id}/estimate?targets=none`, {
        strategy,
        strategyType: 'expanded',
      });
      setDetail(data);
      if (data.strategy?.strategy) setStrategy(data.strategy.strategy);
      setEstimateFresh(data.session.status !== 'estimating');
    } catch (err) {
      setError(errorLabel(err));
      setBusy(null);
    } finally {
      if (detail.session.status !== 'estimating') setBusy(null);
    }
  }

  async function executeSession() {
    if (!detail?.session.id || !strategy || !preview || preview.blocked || preview.count === 0) return;
    setBusy('execute');
    setError(null);
    try {
      await api.post<MASessionDetail>(`/binocolo/v1/ma/sessions/${detail.session.id}/gated-search?targets=none`, {
        strategyType: 'expanded',
        limit: normalizeSearchLimit(strategy.searchLimit),
      });
      navigate(`/ricerche/${detail.session.id}`);
    } catch (err) {
      setError(errorLabel(err));
      toast(errorLabel(err), 'error');
    } finally {
      setBusy(null);
    }
  }

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <span className={styles.eyebrow}>Nuova ricerca</span>
          <h1>Definisci il perimetro</h1>
          <p className={styles.subtitle}>Descrizione del perimetro in linguaggio naturale. Il sistema riconosce ambiti di attività, territorio e vincoli.</p>
        </div>
        <Button variant="secondary" onClick={() => navigate('/ricerche')} leftIcon={<Icon name="arrow-left" />}>
          Ricerche
        </Button>
      </header>

      {error ? (
        <div className={styles.danger} role="alert">
          <Icon name="triangle-alert" size={18} />
          <span>{error}</span>
        </div>
      ) : null}

      {showRequestEditor || !detail || !strategy ? (
        <section className={styles.panel} aria-labelledby="request-title">
          <div className={styles.panelHeader}>
            <div>
              <h2 id="request-title">Richiesta in linguaggio naturale</h2>
              <p className={styles.hint}>Ogni ricerca nasce dentro un'iniziativa. Il percorso rapido genera il titolo dal perimetro.</p>
            </div>
          </div>
          <form className={styles.panelBody} onSubmit={createSession}>
            <fieldset className={styles.initiativeSelector} aria-describedby="initiative-help">
              <legend className={styles.label}>Iniziativa</legend>
              <div className={styles.initiativeChoices}>
                <label className={`${styles.initiativeChoice} ${initiativeMode === 'auto' ? styles.initiativeChoiceActive : ''}`}>
                  <input
                    type="radio"
                    name="initiativeMode"
                    value="auto"
                    checked={initiativeMode === 'auto'}
                    onChange={() => {
                      initiativeSelectionTouchedRef.current = true;
                      setInitiativeMode('auto');
                      setInitiativeError(null);
                    }}
                  />
                  <span>
                    <b>Nuova iniziativa · titolo automatico</b>
                    <small>Il titolo viene generato dalla ricerca (prefisso TGT).</small>
                  </span>
                </label>
                <label className={`${styles.initiativeChoice} ${initiativeMode === 'manual' ? styles.initiativeChoiceActive : ''}`}>
                  <input
                    type="radio"
                    name="initiativeMode"
                    value="manual"
                    checked={initiativeMode === 'manual'}
                    onChange={() => {
                      initiativeSelectionTouchedRef.current = true;
                      setInitiativeMode('manual');
                      setInitiativeError(null);
                    }}
                  />
                  <span>
                    <b>Nuova iniziativa · titolo manuale</b>
                    <small>Usare un nome già deciso per il filone di scouting.</small>
                  </span>
                </label>
                <label className={`${styles.initiativeChoice} ${initiativeMode === 'existing' ? styles.initiativeChoiceActive : ''}`}>
                  <input
                    type="radio"
                    name="initiativeMode"
                    value="existing"
                    checked={initiativeMode === 'existing'}
                    onChange={() => {
                      initiativeSelectionTouchedRef.current = true;
                      setInitiativeMode('existing');
                      setInitiativeError(null);
                      if (!initiativeId && activeInitiatives[0]) setInitiativeId(activeInitiatives[0].id);
                    }}
                    disabled={initiativesLoading || activeInitiatives.length === 0}
                  />
                  <span>
                    <b>Iniziativa esistente</b>
                    <small>{initiativesLoading ? 'Caricamento iniziative…' : activeInitiatives.length > 0 ? 'Collega la ricerca a un filone già aperto.' : 'Non ci sono iniziative attive disponibili.'}</small>
                  </span>
                </label>
              </div>
              {initiativeMode === 'manual' ? (
                <label className={styles.field}>
                  <span className={styles.label}>Titolo iniziativa</span>
                  <input
                    className={`${styles.input} ${initiativeError ? styles.inputError : ''}`}
                    value={manualInitiativeTitle}
                    onChange={(event) => {
                      initiativeSelectionTouchedRef.current = true;
                      setManualInitiativeTitle(event.target.value);
                      if (initiativeError) setInitiativeError(null);
                    }}
                    placeholder="es. Acquisizione MSP 2026"
                    maxLength={120}
                    required
                    aria-invalid={Boolean(initiativeError)}
                    aria-describedby={initiativeError ? 'initiative-error' : 'initiative-help'}
                  />
                </label>
              ) : null}
              {initiativeMode === 'existing' ? (
                <label className={styles.field}>
                  <span className={styles.label}>Iniziativa attiva</span>
                  <select
                    className={`${styles.select} ${initiativeError ? styles.inputError : ''}`}
                    value={initiativeId}
                    onChange={(event) => {
                      initiativeSelectionTouchedRef.current = true;
                      setInitiativeId(event.target.value);
                      if (initiativeError) setInitiativeError(null);
                    }}
                    disabled={initiativesLoading || activeInitiatives.length === 0}
                    aria-invalid={Boolean(initiativeError)}
                    aria-describedby={initiativeError ? 'initiative-error' : 'initiative-help'}
                  >
                    {activeInitiatives.map((item) => (
                      <option key={item.id} value={item.id}>{item.title}</option>
                    ))}
                  </select>
                </label>
              ) : null}
              {initiativeError ? <p className={styles.fieldError} id="initiative-error">{initiativeError}</p> : null}
              <p className={styles.hint} id="initiative-help">Le aziende con almeno una stella entreranno nella lavorazione dell'iniziativa.</p>
            </fieldset>

            <label className={styles.stack}>
              <span className={styles.label}>
                <span className={styles.dotRequired} aria-hidden="true" />
                Richiesta
              </span>
              <textarea
                className={styles.textarea}
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                placeholder="Descrivi in dettaglio le attività delle aziende cercate: cosa fanno, per quali clienti, con quali tecnologie o servizi. Aggiungi territorio e vincoli (fatturato, dipendenti, età del titolare…)."
                rows={8}
              />
            </label>
            <div className={styles.guide}>
              <span>
                <b>Più la descrizione è ricca, più la ricerca è mirata.</b> Il sistema valuta cosa fanno davvero le aziende, non solo la classificazione formale.
              </span>
              <span>Non servono codici ATECO: descrivere gli ambiti di business in modo discorsivo.</span>
              <span>
                Esempio: «Aziende che sviluppano software gestionale per la logistica e l'automazione di magazzino, anche system integrator specializzati; escluse le web agency. Province di Milano e Monza, fatturato tra 1 e 5 milioni, titolare vicino alla pensione.»
              </span>
            </div>
            <div className={styles.actions}>
              <Button
                type="submit"
                loading={busy === 'create'}
                disabled={prompt.trim().length < minPromptLength}
                leftIcon={<Icon name="sparkles" />}
              >
                Analizza la richiesta
              </Button>
              <span className={styles.hint}>Attivo con una descrizione sufficiente</span>
            </div>
          </form>
        </section>
      ) : (
        <>
          <section className={styles.panel}>
            <div className={styles.panelBody}>
              <div className={styles.collapsedPrompt}>
                <div className={styles.collapsedSummary}>
                  <p>
                    <b>Richiesta.</b> «{detail.session.prompt}»
                  </p>
                  {detail.session.initiativeTitle ? (
                    <p>
                      <b>Iniziativa.</b> {detail.session.initiativeTitle}{' '}
                      <span className={styles.hint}>Titolo modificabile dalla pagina iniziative.</span>
                    </p>
                  ) : null}
                </div>
                <button
                  type="button"
                  className={styles.linkButton}
                  onClick={() => {
                    setPrompt(detail.session.prompt);
                    setShowRequestEditor(true);
                  }}
                >
                  Modifica
                </button>
              </div>
            </div>
          </section>

          <div className={styles.grid}>
            <div className={styles.stack}>
              <section className={styles.panel} aria-labelledby="ambiti-title">
                <div className={styles.panelHeader}>
                  <div>
                    <h2 id="ambiti-title">Ambiti riconosciuti</h2>
                    <p className={styles.hint}>Concetti e superficie settoriale derivati dalla richiesta.</p>
                  </div>
                </div>
                <div className={styles.panelBody}>
                  {strategy.sectorRetrievalMode === 'fallback_llm' ? (
                    <div className={styles.warning}>
                      <Icon name="triangle-alert" size={18} />
                      <span>Perimetro derivato dal catalogo ATECO.</span>
                    </div>
                  ) : null}
                  {blocker ? (
                    <div className={styles.stack}>
                      <div className={styles.danger}>
                        <Icon name="triangle-alert" size={18} />
                        <span>
                          <b>{blocker.title}</b> {blocker.message}
                        </span>
                      </div>
                      <Button variant="secondary" onClick={() => setShowRequestEditor(true)} leftIcon={<Icon name="pencil" />}>
                        Modifica la richiesta
                      </Button>
                    </div>
                  ) : (
                    <ConceptSummary
                      strategy={strategy}
                      open={atecoOpen}
                      onToggle={() => setAtecoOpen((current) => !current)}
                    />
                  )}
                </div>
              </section>

              <section className={styles.panel} aria-labelledby="territorio-title">
                <div className={styles.panelHeader}>
                  <div>
                    <h2 id="territorio-title">Territorio</h2>
                    <p className={styles.hint}>Recap sintetico delle province incluse nella stima.</p>
                  </div>
                  <Button variant="secondary" size="sm" onClick={() => setProvinceModalOpen(true)} leftIcon={<Icon name="pencil" size={14} />}>
                    Modifica province
                  </Button>
                </div>
                <div className={styles.panelBody}>
                  <div className={styles.territoryBox}>
                    <Icon name="landmark" size={18} />
                    <span className={styles.territoryValue}>{summarizeTerritory(strategy.provinces, catalog)}</span>
                    <span className={styles.hint}>
                      {strategy.provinces.length === 0 ? 'tutte le province' : `${numberFormat.format(strategy.provinces.length)} province`}
                    </span>
                  </div>
                  {wideTerritory ? (
                    <div className={styles.warning}>
                      <Icon name="triangle-alert" size={18} />
                      <span>
                        <b>Perimetro territoriale ampio.</b> La stima sarà più lenta e la ricerca meno mirata. Preferire una regione o poche province.
                      </span>
                    </div>
                  ) : null}
                </div>
              </section>

              <section className={styles.panel} aria-labelledby="vincoli-title">
                <div className={styles.panelHeader}>
                  <div>
                    <h2 id="vincoli-title">Vincoli</h2>
                    <p className={styles.hint}>Modificare un vincolo richiede una nuova stima.</p>
                  </div>
                </div>
                <div className={styles.panelBody}>
                  <ConstraintEditor strategy={strategy} onChange={updateStrategy} />
                </div>
              </section>
            </div>

            <aside className={styles.stack}>
              <section className={styles.panel} aria-labelledby="stima-title">
                <div className={styles.panelHeader}>
                  <div>
                    <h2 id="stima-title">Stima della ricerca</h2>
                    <p className={styles.hint}>Solo superficie espansa, pronta per il gate.</p>
                  </div>
                </div>
                <div className={styles.panelBody}>
                  {blocker ? (
                    <div className={styles.infoNotice}>
                      <Icon name="info" size={18} />
                      <span>La stima sarà disponibile dopo aver completato gli ambiti cercati.</span>
                    </div>
                  ) : estimating ? (
                    <Skeleton rows={6} />
                  ) : preview ? (
                    <EstimatePanel
                      estimates={preview.rows}
                      count={preview.count}
                      blocked={preview.blocked}
                      lowerBound={preview.lowerBound}
                      catalog={catalog}
                      onRemoveProvince={(province) => {
                        updateStrategy({ provinces: strategy.provinces.filter((code) => code !== province) });
                      }}
                      onExecute={() => void executeSession()}
                      executeBusy={busy === 'execute'}
                    />
                  ) : (
                    <div className={styles.stack}>
                      {!estimateFresh && (detail.estimates?.length ?? 0) > 0 ? (
                        <div className={styles.warning}>
                          <Icon name="triangle-alert" size={18} />
                          <span>Il perimetro è cambiato dopo la stima. Calcola una nuova preview prima di confermare.</span>
                        </div>
                      ) : null}
                      <div className={styles.infoNotice}>
                        <Icon name="search" size={18} />
                        <span>La stima richiede qualche istante.</span>
                      </div>
                    </div>
                  )}
                  {!blocker && !preview ? (
                    <div className={styles.actions}>
                      <Button onClick={() => void estimateSession()} loading={busy === 'estimate'} disabled={!canEstimate} leftIcon={<Icon name="search" />}>
                        Calcola stima
                      </Button>
                      {wideTerritory ? <span className={styles.hint}>Preferire una regione o poche province.</span> : null}
                    </div>
                  ) : null}
                </div>
              </section>
            </aside>
          </div>
        </>
      )}

      {strategy ? (
        <ProvinceModal
          open={provinceModalOpen}
          catalog={catalog}
          selected={strategy.provinces}
          onClose={() => setProvinceModalOpen(false)}
          onApply={(provinces) => {
            updateStrategy({ provinces });
            setProvinceModalOpen(false);
          }}
        />
      ) : null}

    </main>
  );
}

function ConceptSummary({ strategy, open, onToggle }: { strategy: MAStrategySpec; open: boolean; onToggle: () => void }) {
  const concepts = strategy.sectorConcepts ?? [];
  const divisionGroups = useMemo(() => groupsFromStrategy(strategy), [strategy]);

  return (
    <div className={styles.stack}>
      {concepts.length > 0 ? (
        <div className={styles.chips}>
          {concepts.map((concept) => (
            <span key={concept.id || concept.name} className={`${styles.chip} ${concept.fit === 'weak' ? styles.chipWeak : styles.chipCore}`}>
              {concept.name}
              <small>{concept.fit === 'weak' ? 'adiacente' : 'core'}</small>
            </span>
          ))}
        </div>
      ) : (
        <p className={styles.hint}>Ambiti derivati dal catalogo settoriale.</p>
      )}

      <button type="button" className={styles.toggleButton} onClick={onToggle} aria-expanded={open}>
        <Icon name={open ? 'chevron-down' : 'chevron-right'} size={14} /> Settori ATECO utilizzati
      </button>
      {open ? (
        <div className={styles.chevronPanel}>
          {divisionGroups.length === 0 ? (
            <p className={styles.hint}>Nessun codice ATECO esplicito nel perimetro.</p>
          ) : (
            divisionGroups.map((group) => (
              <div key={group.division} className={styles.divisionRow}>
                <span className={styles.divisionCode}>{group.division}</span>
                <span>
                  <b>{group.label}</b>
                  <span className={styles.chips}>
                    {group.codes.map((code) => (
                      <span key={code} className={styles.code}>{code}</span>
                    ))}
                  </span>
                </span>
              </div>
            ))
          )}
          <p className={styles.hint}>La ricerca esplora le divisioni per intero; il giudizio di aderenza avviene sull'attività reale.</p>
        </div>
      ) : null}

      <span className={styles.thesis}>
        Tesi: {thesisLabels[(strategy.thesis ?? 'unknown') as MAThesis | 'unknown']}
        <small>· derivata dalla richiesta, modificabile dopo l'esecuzione</small>
      </span>
    </div>
  );
}

function ConstraintEditor({ strategy, onChange }: { strategy: MAStrategySpec; onChange: (patch: Partial<MAStrategySpec>) => void }) {
  const thesis = strategy.thesis ?? 'generico';
  const legalFormsValue = (strategy.legalForms ?? []).join(', ');
  const updateNumber =
    (field: keyof Pick<MAStrategySpec, 'turnoverMin' | 'turnoverMax' | 'employeeMin' | 'employeeMax' | 'revenuePerEmployeeMin' | 'maxShareholders' | 'successionMinOwnerAge'>) =>
    (event: ChangeEvent<HTMLInputElement>) => {
      onChange({ [field]: optionalNumber(event.target.value) } as Partial<MAStrategySpec>);
    };

  return (
    <div className={styles.fields}>
      <RangeField
        label="Fatturato (€)"
        minValue={strategy.turnoverMin}
        maxValue={strategy.turnoverMax}
        onMin={updateNumber('turnoverMin')}
        onMax={updateNumber('turnoverMax')}
      />
      <RangeField
        label="Dipendenti"
        minValue={strategy.employeeMin}
        maxValue={strategy.employeeMax}
        onMin={updateNumber('employeeMin')}
        onMax={updateNumber('employeeMax')}
      />
      <label className={styles.field}>
        <span>Ricavo minimo / dipendente (€)</span>
        <input
          className={styles.input}
          type="number"
          min={0}
          value={strategy.revenuePerEmployeeMin ?? ''}
          onChange={updateNumber('revenuePerEmployeeMin')}
          placeholder="min"
        />
      </label>
      <label className={styles.field}>
        <span>Numero massimo soci</span>
        <input
          className={styles.input}
          type="number"
          min={1}
          value={strategy.maxShareholders ?? ''}
          onChange={updateNumber('maxShareholders')}
          placeholder="max"
        />
      </label>
      <label className={styles.field}>
        <span>Forme giuridiche</span>
        <input
          className={styles.input}
          value={legalFormsValue}
          onChange={(event) =>
            onChange({
              legalForms: event.target.value
                .split(',')
                .map((item) => item.trim().toUpperCase())
                .filter(Boolean),
            })
          }
          placeholder="SRL, SPA"
        />
      </label>
      {(thesis === 'successione' || strategy.successionMinOwnerAge != null) ? (
        <label className={styles.field}>
          <span>Età minima titolare</span>
          <input
            className={styles.input}
            type="number"
            min={30}
            max={90}
            value={strategy.successionMinOwnerAge ?? ''}
            onChange={updateNumber('successionMinOwnerAge')}
            placeholder="60"
          />
        </label>
      ) : null}
      <label className={styles.field}>
        <span>Numero risultati</span>
        <input
          className={styles.input}
          type="number"
          min={1}
          max={1000}
          value={normalizeSearchLimit(strategy.searchLimit)}
          onChange={(event) => onChange({ searchLimit: normalizeSearchLimit(optionalNumber(event.target.value)) })}
        />
      </label>
    </div>
  );
}

function RangeField({
  label,
  minValue,
  maxValue,
  onMin,
  onMax,
}: {
  label: string;
  minValue?: number;
  maxValue?: number;
  onMin: (event: ChangeEvent<HTMLInputElement>) => void;
  onMax: (event: ChangeEvent<HTMLInputElement>) => void;
}) {
  return (
    <label className={styles.field}>
      <span>{label}</span>
      <span className={styles.fieldRow}>
        <input className={styles.input} type="number" min={0} value={minValue ?? ''} onChange={onMin} placeholder="min" />
        <span className={styles.hint}>-</span>
        <input className={styles.input} type="number" min={0} value={maxValue ?? ''} onChange={onMax} placeholder="max" />
      </span>
    </label>
  );
}

function EstimatePanel({
  estimates,
  count,
  blocked,
  lowerBound,
  catalog,
  onRemoveProvince,
  onExecute,
  executeBusy,
}: {
  estimates: MAEstimate[];
  count: number;
  blocked: boolean;
  lowerBound: boolean;
  catalog: MAProvinceCatalogItem[];
  onRemoveProvince: (province: string) => void;
  onExecute: () => void;
  executeBusy: boolean;
}) {
  const rows = [...estimates].sort((a, b) => b.estimatedCount - a.estimatedCount);
  const max = Math.max(1, ...rows.map((row) => row.estimatedCount));

  if (count === 0) {
    return (
      <div className={styles.emptyState}>
        <span className={styles.emptyIcon}><Icon name="search" size={28} /></span>
        <strong>Nessuna azienda nel perimetro</strong>
        <p className={styles.hint}>Ampliare gli ambiti di attività descritti nella richiesta, oppure estendere il territorio.</p>
      </div>
    );
  }

  return (
    <div className={styles.stack}>
      <div className={styles.estimateNumber}>
        {estimateCountLabel(count, lowerBound)} <small>aziende nel perimetro</small>
      </div>
      {blocked ? (
        <div className={styles.danger}>
          <Icon name="triangle-alert" size={18} />
          <span>
            <b>Il perimetro supera la capacità di una ricerca completa.</b> Restringere il territorio o gli ambiti di attività per procedere.
          </span>
        </div>
      ) : null}
      <div className={styles.provinceBars}>
        {rows.map((estimate) => {
          const width = Math.max(4, Math.round((estimate.estimatedCount / max) * 100));
          const province = estimate.province ?? '';
          return (
            <div key={estimate.id} className={styles.provinceBar}>
              <span>{provinceLabel(province, catalog)}</span>
              <span className={styles.barTrack}>
                <span className={styles.barFill} style={{ width: `${width}%` }} />
              </span>
              <b>{estimateCountLabel(estimate.estimatedCount, estimateUsesLowerBound(estimate))}</b>
              {province ? (
                <button type="button" className={styles.iconButton} onClick={() => onRemoveProvince(province)} aria-label={`Rimuovi ${province}`}>
                  <Icon name="x" size={14} />
                </button>
              ) : <span />}
            </div>
          );
        })}
      </div>
      <p className={styles.hint}>Ripartizione per provincia. Rimuovere una provincia aggiorna il perimetro e richiede una nuova stima.</p>
      <div className={styles.actions}>
        {!blocked ? (
          <Button onClick={onExecute} loading={executeBusy} leftIcon={<Icon name="check" />}>
            Conferma e avvia
          </Button>
        ) : null}
        <span className={styles.hint}>L'analisi è asincrona: l'avanzamento è consultabile nella pagina della ricerca.</span>
      </div>
    </div>
  );
}

function ProvinceModal({
  open,
  catalog,
  selected,
  onClose,
  onApply,
}: {
  open: boolean;
  catalog: MAProvinceCatalogItem[];
  selected: string[];
  onClose: () => void;
  onApply: (selected: string[]) => void;
}) {
  const [draft, setDraft] = useState<string[]>(selected);

  useEffect(() => {
    if (open) setDraft(selected);
  }, [open, selected]);

  const options = catalog.map((item) => ({ value: item.code, label: `${item.name} (${item.code})` }));
  return (
    <Modal open={open} onClose={onClose} title="Province" size="md">
      <div className={styles.stack}>
        <MultiSelect options={options} selected={draft} onChange={setDraft} placeholder="Seleziona province" />
        <div className={styles.modalActions}>
          <Button onClick={() => onApply(draft)}>Applica</Button>
          <Button variant="secondary" onClick={onClose}>Annulla</Button>
        </div>
      </div>
    </Modal>
  );
}

function groupsFromStrategy(strategy: MAStrategySpec): Array<{ division: string; label: string; codes: string[] }> {
  const groups = new Map<string, { label: string; codes: Set<string> }>();
  for (const concept of strategy.sectorConcepts ?? []) {
    addConceptGroups(groups, concept);
  }
  for (const candidate of strategy.atecoCandidates ?? []) {
    addCandidateGroup(groups, candidate);
  }
  return [...groups.entries()]
    .map(([division, item]) => ({ division, label: item.label, codes: [...item.codes].sort() }))
    .sort((a, b) => a.division.localeCompare(b.division));
}

function addConceptGroups(groups: Map<string, { label: string; codes: Set<string> }>, concept: MAStrategyConcept) {
  for (const division of concept.divisions ?? []) {
    const current = groups.get(division) ?? { label: `Divisione ${division}`, codes: new Set<string>() };
    for (const code of concept.atecoCodes ?? []) {
      if (code.startsWith(division)) current.codes.add(code);
    }
    groups.set(division, current);
  }
}

function addCandidateGroup(groups: Map<string, { label: string; codes: Set<string> }>, candidate: MAAtecoCandidate) {
  const division = candidate.code.slice(0, 2);
  if (!division) return;
  const current = groups.get(division) ?? { label: candidate.description || `Divisione ${division}`, codes: new Set<string>() };
  current.codes.add(candidate.code);
  groups.set(division, current);
}
