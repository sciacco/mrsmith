import { Button, Icon, Modal, MultiSelect, SearchInput, useToast } from '@mrsmith/ui';
import {
  useCallback,
  useDeferredValue,
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
  type KeyboardEvent,
} from 'react';
import { useNavigate } from 'react-router-dom';
import { DealCardSkeleton } from '../components/DealCardSkeleton';
import { Pagination } from '../components/Pagination';
import { useCreateRichiesta, useDeals, useFornitori } from '../api/queries';
import { copyErrorMessage } from '../lib/format';
import styles from './shared.module.css';

type SelectedDeal = {
  id: number;
  codice: string;
  deal_name: string;
  company_name: string | null;
  owner_email: string | null;
  stage_label: string | null;
};

const isMac = typeof navigator !== 'undefined' && /Mac|iPad|iPhone/i.test(navigator.platform);

export function NewRequestPage() {
  const navigate = useNavigate();
  const { toast } = useToast();

  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const [selectedDeal, setSelectedDeal] = useState<SelectedDeal | null>(null);
  const [indirizzo, setIndirizzo] = useState('');
  const [descrizione, setDescrizione] = useState('');
  const [fornitoriPreferiti, setFornitoriPreferiti] = useState<number[]>([]);
  const [touched, setTouched] = useState({ indirizzo: false, descrizione: false });
  const [attemptedSubmit, setAttemptedSubmit] = useState(false);

  const [focusedDealIndex, setFocusedDealIndex] = useState(0);
  const [pulseChip, setPulseChip] = useState(false);

  const [resetConfirmOpen, setResetConfirmOpen] = useState(false);
  const [pendingDeal, setPendingDeal] = useState<SelectedDeal | null>(null);

  const deferredSearch = useDeferredValue(search);
  const deals = useDeals({ q: deferredSearch || undefined, page, page_size: 8 });
  const fornitori = useFornitori();
  const createRichiesta = useCreateRichiesta();

  const indirizzoRef = useRef<HTMLTextAreaElement | null>(null);
  const searchWrapperRef = useRef<HTMLDivElement | null>(null);
  const dealButtonsRef = useRef<Array<HTMLButtonElement | null>>([]);

  const hasFormContent =
    indirizzo.trim() !== '' || descrizione.trim() !== '' || fornitoriPreferiti.length > 0;
  const isDirty = selectedDeal !== null || hasFormContent;
  const isSubmitting = createRichiesta.isPending;

  const missing = useMemo(() => {
    const reasons: string[] = [];
    if (!selectedDeal) reasons.push('scegli un deal');
    if (indirizzo.trim() === '') reasons.push('indirizzo');
    if (descrizione.trim() === '') reasons.push('descrizione');
    return reasons;
  }, [selectedDeal, indirizzo, descrizione]);

  const canSubmit = missing.length === 0;

  const dealsItems = deals.data?.items ?? [];
  const totalPages = deals.data
    ? Math.max(1, Math.ceil(deals.data.total / deals.data.page_size))
    : 1;

  useEffect(() => {
    dealButtonsRef.current = dealButtonsRef.current.slice(0, dealsItems.length);
    if (focusedDealIndex >= dealsItems.length) {
      setFocusedDealIndex(dealsItems.length > 0 ? dealsItems.length - 1 : 0);
    }
  }, [dealsItems.length, focusedDealIndex]);

  useEffect(() => {
    if (!selectedDeal) return;
    setPulseChip(true);
    const timeout = window.setTimeout(() => setPulseChip(false), 700);
    return () => window.clearTimeout(timeout);
  }, [selectedDeal?.id]);

  const commitDealSelection = useCallback((deal: SelectedDeal) => {
    setSelectedDeal(deal);
    window.requestAnimationFrame(() => {
      indirizzoRef.current?.focus({ preventScroll: false });
      if (window.matchMedia('(max-width: 1000px)').matches) {
        indirizzoRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' });
      }
    });
  }, []);

  const onPickDeal = useCallback(
    (deal: SelectedDeal) => {
      if (selectedDeal && selectedDeal.id !== deal.id && hasFormContent) {
        setPendingDeal(deal);
        return;
      }
      commitDealSelection(deal);
    },
    [commitDealSelection, hasFormContent, selectedDeal],
  );

  const confirmSwapDeal = () => {
    if (!pendingDeal) return;
    const target = pendingDeal;
    setPendingDeal(null);
    commitDealSelection(target);
  };

  const clearAll = useCallback(() => {
    setSelectedDeal(null);
    setIndirizzo('');
    setDescrizione('');
    setFornitoriPreferiti([]);
    setTouched({ indirizzo: false, descrizione: false });
    setAttemptedSubmit(false);
  }, []);

  const onResetClick = () => {
    if (isDirty) {
      setResetConfirmOpen(true);
      return;
    }
    clearAll();
  };

  const confirmReset = () => {
    clearAll();
    setResetConfirmOpen(false);
    toast('Form svuotato');
  };

  useEffect(() => {
    if (!isDirty || isSubmitting) return;
    const handler = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [isDirty, isSubmitting]);

  const handleSubmit = useCallback(
    (event?: FormEvent<HTMLFormElement>) => {
      event?.preventDefault();
      setAttemptedSubmit(true);
      setTouched({ indirizzo: true, descrizione: true });
      if (!canSubmit || !selectedDeal) return;
      createRichiesta.mutate(
        {
          deal_id: selectedDeal.id,
          indirizzo: indirizzo.trim(),
          descrizione: descrizione.trim(),
          fornitori_preferiti: fornitoriPreferiti,
        },
        {
          onSuccess: (created) => {
            toast(`Richiesta #${created.id} creata`);
            clearAll();
            navigate(`/richieste/${created.id}/view`);
          },
          onError: (error) => {
            toast(copyErrorMessage(error, 'Impossibile creare la richiesta.'), 'error');
          },
        },
      );
    },
    [canSubmit, clearAll, createRichiesta, descrizione, fornitoriPreferiti, indirizzo, navigate, selectedDeal, toast],
  );

  useEffect(() => {
    const onKey = (event: globalThis.KeyboardEvent) => {
      const meta = event.metaKey || event.ctrlKey;
      if (meta && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        const input = searchWrapperRef.current?.querySelector('input');
        input?.focus();
        input?.select();
      }
      if (meta && event.key === 'Enter' && canSubmit && !isSubmitting) {
        event.preventDefault();
        handleSubmit();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [canSubmit, handleSubmit, isSubmitting]);

  const onDealListKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (dealsItems.length === 0) return;
    let nextIndex = focusedDealIndex;
    switch (event.key) {
      case 'ArrowDown':
      case 'ArrowRight':
        nextIndex = (focusedDealIndex + 1) % dealsItems.length;
        break;
      case 'ArrowUp':
      case 'ArrowLeft':
        nextIndex = (focusedDealIndex - 1 + dealsItems.length) % dealsItems.length;
        break;
      case 'Home':
        nextIndex = 0;
        break;
      case 'End':
        nextIndex = dealsItems.length - 1;
        break;
      default:
        return;
    }
    event.preventDefault();
    setFocusedDealIndex(nextIndex);
    dealButtonsRef.current[nextIndex]?.focus();
  };

  const showIndirizzoError = (touched.indirizzo || attemptedSubmit) && indirizzo.trim() === '';
  const showDescrizioneError = (touched.descrizione || attemptedSubmit) && descrizione.trim() === '';
  const showNoDealError = attemptedSubmit && !selectedDeal;

  const canSubmitHint =
    missing.length === 0
      ? 'Tutto pronto per inserire la RDF.'
      : `Per completare: ${missing.join(', ')}.`;

  const shortcutLabel = isMac ? '⌘' : 'Ctrl';

  return (
    <section className={styles.page}>
      <div className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>Nuova RDF</h1>
          <p className={styles.pageSubtitle}>
            Cerca il deal corretto, seleziona i fornitori preferiti e inserisci il contesto operativo della richiesta.
          </p>
        </div>
        <div className={styles.headerActions}>
          <span className={styles.shortcutHint} aria-hidden="true">
            <span className={styles.kbd}>{shortcutLabel}</span>
            <span className={styles.kbd}>K</span>
            cerca deal
          </span>
        </div>
      </div>

      <div className={styles.newGrid}>
        <aside className={`${styles.panel} ${styles.newPanelSticky}`}>
          <div className={styles.panelHeader}>
            <div>
              <h2 className={styles.panelTitle} id="deal-group-heading">Selezione deal</h2>
              <p className={styles.small}>Cerca per codice, nome deal, cliente o owner.</p>
            </div>
          </div>

          <div className={styles.filterStack} ref={searchWrapperRef}>
            <SearchInput
              value={search}
              onChange={(value) => {
                setSearch(value);
                setPage(1);
                setFocusedDealIndex(0);
              }}
              placeholder="Cerca deal..."
            />
          </div>

          <div
            className={styles.dealListScroll}
            role="radiogroup"
            aria-labelledby="deal-group-heading"
            aria-invalid={showNoDealError || undefined}
            onKeyDown={onDealListKeyDown}
          >
            <div className={styles.dealList}>
              {deals.isLoading ? (
                <DealCardSkeleton rows={4} />
              ) : deals.error ? (
                <div className={styles.emptyCard}>
                  <div className={styles.emptyIconDanger}><Icon name="triangle-alert" /></div>
                  <h3>Deal non disponibili</h3>
                  <p className={styles.muted}>{copyErrorMessage(deals.error, 'Impossibile caricare i deal.')}</p>
                  <div className={styles.retryRow}>
                    <Button variant="secondary" onClick={() => deals.refetch()} loading={deals.isFetching}>
                      Riprova
                    </Button>
                  </div>
                </div>
              ) : dealsItems.length === 0 ? (
                <div className={styles.emptyCard}>
                  <div className={styles.emptyIcon}><Icon name="search" /></div>
                  <h3>Nessun deal trovato</h3>
                  <p className={styles.muted}>Affina la ricerca per trovare il deal corretto.</p>
                </div>
              ) : (
                <>
                  {dealsItems.map((deal, index) => {
                    const selected = selectedDeal?.id === deal.id;
                    const isRovingFocus = index === focusedDealIndex;
                    return (
                      <button
                        key={deal.id}
                        ref={(node) => {
                          dealButtonsRef.current[index] = node;
                        }}
                        type="button"
                        role="radio"
                        aria-checked={selected}
                        tabIndex={isRovingFocus ? 0 : -1}
                        className={`${styles.dealCard} ${selected ? styles.dealCardSelected : ''}`}
                        onClick={() => {
                          setFocusedDealIndex(index);
                          onPickDeal({
                            id: deal.id,
                            codice: deal.codice,
                            deal_name: deal.deal_name,
                            company_name: deal.company_name,
                            owner_email: deal.owner_email,
                            stage_label: deal.stage_label,
                          });
                        }}
                        onFocus={() => setFocusedDealIndex(index)}
                      >
                        <div className={styles.listItemTop}>
                          <div>
                            <div className={styles.summaryCode}>{deal.codice}</div>
                            <div className={styles.listHeading}>{deal.company_name ?? 'Cliente non disponibile'}</div>
                          </div>
                          <div className={styles.small}>{deal.stage_label ?? 'Stage non disponibile'}</div>
                        </div>
                        <p>{deal.deal_name}</p>
                        <p className={styles.small}>{deal.owner_email ?? 'Owner non disponibile'}</p>
                      </button>
                    );
                  })}
                </>
              )}
            </div>
          </div>

          {deals.data && deals.data.total > deals.data.page_size && (
            <Pagination
              page={deals.data.page}
              pageSize={deals.data.page_size}
              total={deals.data.total}
              label={deals.data.total === 1 ? 'deal' : 'deal'}
              onPageChange={(nextPage) => {
                setPage(Math.min(Math.max(1, nextPage), totalPages));
                setFocusedDealIndex(0);
              }}
            />
          )}
        </aside>

        <form className={styles.formStack} onSubmit={handleSubmit} noValidate>
          {selectedDeal ? (
            <div
              className={`${styles.dealChip} ${pulseChip ? styles.dealChipPulse : ''}`}
              aria-live="polite"
            >
              <div className={styles.dealChipBody}>
                <div className={styles.summaryCode}>{selectedDeal.codice}</div>
                <div className={styles.dealChipTitle}>
                  {selectedDeal.company_name ?? 'Cliente non disponibile'}
                </div>
                <div className={styles.dealChipMeta}>
                  <span>{selectedDeal.deal_name}</span>
                  {selectedDeal.owner_email && <span>{selectedDeal.owner_email}</span>}
                  {selectedDeal.stage_label && <span>{selectedDeal.stage_label}</span>}
                </div>
              </div>
              <div className={styles.dealChipActions}>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => setSelectedDeal(null)}
                  aria-label="Rimuovi deal selezionato"
                >
                  Cambia
                </Button>
              </div>
            </div>
          ) : (
            <div className={`${styles.emptyCard} ${showNoDealError ? styles.fieldInputInvalid : ''}`}>
              <div className={styles.emptyIcon}><Icon name="package" /></div>
              <h3>Seleziona un deal</h3>
              <p className={styles.muted}>
                La richiesta può essere inserita solo dopo aver scelto un deal eleggibile.
              </p>
              {showNoDealError && (
                <p className={styles.fieldError}>Serve un deal per proseguire.</p>
              )}
            </div>
          )}

          <div className={styles.formCard}>
            <div className={styles.panelHeader}>
              <div>
                <h2 className={styles.panelTitle}>Dati richiesta</h2>
                <p className={styles.small}>Prepara il contesto per il team carrier.</p>
              </div>
            </div>

            <div className={styles.formGrid}>
              <div className={styles.fieldGroup}>
                <label htmlFor="indirizzo">
                  Indirizzo<span className={styles.requiredMark} aria-hidden="true">*</span>
                </label>
                <textarea
                  id="indirizzo"
                  ref={indirizzoRef}
                  className={`${styles.textArea} ${styles.addressArea} ${showIndirizzoError ? styles.fieldInputInvalid : ''}`}
                  value={indirizzo}
                  onChange={(event) => setIndirizzo(event.target.value)}
                  onBlur={() => setTouched((prev) => ({ ...prev, indirizzo: true }))}
                  placeholder={'Via, numero civico\nCAP, città, provincia'}
                  required
                  rows={3}
                  aria-invalid={showIndirizzoError || undefined}
                  aria-describedby={showIndirizzoError ? 'indirizzo-error' : undefined}
                />
                {showIndirizzoError && (
                  <p id="indirizzo-error" className={styles.fieldError}>
                    Serve un indirizzo per far partire la fattibilità.
                  </p>
                )}
              </div>

              <div className={styles.fieldGroup}>
                <label htmlFor="descrizione">
                  Descrizione<span className={styles.requiredMark} aria-hidden="true">*</span>
                </label>
                <textarea
                  id="descrizione"
                  className={`${styles.textArea} ${showDescrizioneError ? styles.fieldInputInvalid : ''}`}
                  value={descrizione}
                  onChange={(event) => setDescrizione(event.target.value)}
                  onBlur={() => setTouched((prev) => ({ ...prev, descrizione: true }))}
                  placeholder="Dettagli utili per la fattibilità, vincoli e obiettivi attesi"
                  required
                  aria-invalid={showDescrizioneError || undefined}
                  aria-describedby={showDescrizioneError ? 'descrizione-error' : undefined}
                />
                {showDescrizioneError && (
                  <p id="descrizione-error" className={styles.fieldError}>
                    Aggiungi almeno una riga di contesto: guida la valutazione dei carrier.
                  </p>
                )}
              </div>

              <div className={styles.fieldGroup}>
                <label htmlFor="fornitori-preferiti">
                  Fornitori preferiti
                  <span className={styles.fieldHint}>(opzionale)</span>
                </label>
                <MultiSelect
                  options={(fornitori.data ?? []).map((item) => ({ value: item.id, label: item.nome }))}
                  selected={fornitoriPreferiti}
                  onChange={setFornitoriPreferiti}
                  placeholder={
                    fornitori.isLoading
                      ? 'Caricamento fornitori...'
                      : fornitori.error
                        ? 'Impossibile caricare i fornitori'
                        : (fornitori.data?.length ?? 0) === 0
                          ? 'Nessun fornitore disponibile'
                          : 'Seleziona fornitori'
                  }
                />
                {fornitori.error ? (
                  <p className={styles.fieldError}>
                    {copyErrorMessage(fornitori.error, 'Impossibile caricare i fornitori.')}{' '}
                    <button
                      type="button"
                      className={styles.inlineRetryBtn}
                      onClick={() => fornitori.refetch()}
                    >
                      Riprova
                    </button>
                  </p>
                ) : !fornitori.isLoading && (fornitori.data?.length ?? 0) === 0 ? (
                  <p className={styles.small}>
                    Nessun fornitore presente in anagrafica. Puoi proseguire senza sceglierne.
                  </p>
                ) : null}
              </div>
            </div>
          </div>

          <div className={styles.stickyActionBar}>
            <div className={styles.stickyActionBarHint} aria-live="polite">
              {canSubmitHint}
              {canSubmit && (
                <>
                  {' '}
                  <span className={styles.shortcutHint}>
                    <span className={styles.kbd}>{shortcutLabel}</span>
                    <span className={styles.kbd}>↵</span>
                  </span>
                </>
              )}
            </div>
            <div className={styles.stickyActionBarButtons}>
              <Button type="button" variant="secondary" onClick={onResetClick} disabled={isSubmitting}>
                Reset
              </Button>
              <Button type="submit" loading={isSubmitting} disabled={!canSubmit && !attemptedSubmit}>
                Inserisci RDF
              </Button>
            </div>
          </div>
        </form>
      </div>

      <Modal
        open={resetConfirmOpen}
        onClose={() => setResetConfirmOpen(false)}
        title="Azzerare il form?"
        size="sm"
      >
        <div className={styles.modalBody}>
          <p>Perderai il deal selezionato, l'indirizzo, la descrizione e i fornitori preferiti.</p>
          <div className={styles.actionsRow} style={{ justifyContent: 'flex-end' }}>
            <Button variant="secondary" onClick={() => setResetConfirmOpen(false)}>
              Annulla
            </Button>
            <Button variant="danger" onClick={confirmReset}>
              Sì, azzera
            </Button>
          </div>
        </div>
      </Modal>

      <Modal
        open={pendingDeal !== null}
        onClose={() => setPendingDeal(null)}
        title="Cambiare deal?"
        size="sm"
      >
        <div className={styles.modalBody}>
          <p>
            Hai già iniziato a compilare la richiesta. Cambiando deal manterrai i dati inseriti ma li assocerai al deal{' '}
            <strong>{pendingDeal?.codice}</strong>.
          </p>
          <div className={styles.actionsRow} style={{ justifyContent: 'flex-end' }}>
            <Button variant="secondary" onClick={() => setPendingDeal(null)}>
              Annulla
            </Button>
            <Button onClick={confirmSwapDeal}>Cambia deal</Button>
          </div>
        </div>
      </Modal>

    </section>
  );
}
