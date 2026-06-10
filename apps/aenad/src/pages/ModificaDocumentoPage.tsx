import { Button, Drawer, Icon, Skeleton } from '@mrsmith/ui';
import { useEffect, useState, Fragment, useMemo } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  useDocumentDetails,
  useDocumentRows,
  useUpdateDocument,
  usePaymentMethods,
  type UpdateDocumentPayload,
} from '../api/queries';
import { formatMoney, parseDecimal, trimDecimalZeros } from '../utils/format';
import { ScaricaPdfButton } from '../components/ScaricaPdfButton';
import styles from './ModificaDocumentoPage.module.css';

interface DocumentState {
  Anagr_Nome: string;
  Anagr_Indirizzo: string;
  Anagr_Cap: string;
  Anagr_Citta: string;
  Anagr_Prov: string;
  Anagr_Nazione: string;
  Anagr_CodiceFiscale: string;
  Anagr_PartitaIva: string;
  Anagr_DestNome: string;
  Anagr_DestIndirizzo: string;
  Anagr_DestCap: string;
  Anagr_DestCitta: string;
  Anagr_DestProv: string;
  Anagr_DestNazione: string;
  Pagamento: string;
  Pagam_CoordBancarie: string;
  NoteInterne: string;
  DescDoc: string;
  DataDoc: string;
  NumDoc: string;
}

interface LocalRow {
  IDDocRiga: number;
  CodArticolo: string;
  Desc: string;
  Qta: string;
  Udm: string;
  PrezzoNetto: string;
  Sconti: string;
  isDescriptive?: boolean;
}

// normalizeDecimalInput porta il valore di un input alla stringa decimale del
// payload: separatore punto, vuoto trattato come zero.
function normalizeDecimalInput(value: string): string {
  const normalized = value.trim().replace(',', '.');
  return normalized === '' ? '0' : normalized;
}

export function ModificaDocumentoPage() {
  const { id } = useParams<{ id: string }>();
  const idDoc = parseInt(id ?? '0') || 0;
  const navigate = useNavigate();

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [expandedDescIndex, setExpandedDescIndex] = useState<number | null>(null);

  const docQuery = useDocumentDetails(idDoc, idDoc > 0);
  const rowsQuery = useDocumentRows(idDoc, idDoc > 0);
  const updateMutation = useUpdateDocument();

  // Local Form States
  const [header, setHeader] = useState<DocumentState | null>(null);
  const [rows, setRows] = useState<LocalRow[]>([]);

  // Synchronize queries to local states
  useEffect(() => {
    if (docQuery.data) {
      setHeader({
        Anagr_Nome: docQuery.data.Anagr_Nome ?? '',
        Anagr_Indirizzo: docQuery.data.Anagr_Indirizzo ?? '',
        Anagr_Cap: docQuery.data.Anagr_Cap ?? '',
        Anagr_Citta: docQuery.data.Anagr_Citta ?? '',
        Anagr_Prov: docQuery.data.Anagr_Prov ?? '',
        Anagr_Nazione: docQuery.data.Anagr_Nazione ?? '',
        Anagr_CodiceFiscale: docQuery.data.Anagr_CodiceFiscale ?? '',
        Anagr_PartitaIva: docQuery.data.Anagr_PartitaIva ?? '',
        Anagr_DestNome: docQuery.data.Anagr_DestNome ?? '',
        Anagr_DestIndirizzo: docQuery.data.Anagr_DestIndirizzo ?? '',
        Anagr_DestCap: docQuery.data.Anagr_DestCap ?? '',
        Anagr_DestCitta: docQuery.data.Anagr_DestCitta ?? '',
        Anagr_DestProv: docQuery.data.Anagr_DestProv ?? '',
        Anagr_DestNazione: docQuery.data.Anagr_DestNazione ?? '',
        Pagamento: docQuery.data.Pagamento ?? '',
        Pagam_CoordBancarie: docQuery.data.Pagam_CoordBancarie ?? '',
        NoteInterne: docQuery.data.NoteInterne ?? '',
        DescDoc: docQuery.data.DescDoc ?? '',
        DataDoc: docQuery.data.DataDoc ? docQuery.data.DataDoc.slice(0, 10) : '',
        NumDoc: docQuery.data.NumDoc ?? '',
      });
    }
  }, [docQuery.data]);

  useEffect(() => {
    if (rowsQuery.data) {
      setRows(
        rowsQuery.data.map((row) => {
          const qta = parseDecimal(row.Qta);
          const prezzoNetto = parseDecimal(row.PrezzoNetto);
          const importoNettoRiga = parseDecimal(row.ImportoNettoRiga);
          const isDescriptive =
            !row.CodArticolo &&
            (qta === null || qta === 0) &&
            row.Desc &&
            (prezzoNetto === null || prezzoNetto === 0) &&
            (importoNettoRiga === null || importoNettoRiga === 0);

          return {
            IDDocRiga: row.IDDocRiga,
            CodArticolo: row.CodArticolo ?? '',
            Desc: row.Desc ?? '',
            Qta: row.Qta ? trimDecimalZeros(row.Qta) : '0',
            Udm: row.Udm ?? '',
            PrezzoNetto: row.PrezzoNetto ? trimDecimalZeros(row.PrezzoNetto) : '0',
            Sconti: row.Sconti ?? '',
            isDescriptive: Boolean(isDescriptive),
          };
        }),
      );
    }
  }, [rowsQuery.data]);

  if (docQuery.isLoading || rowsQuery.isLoading) {
    return (
      <div className={styles.page}>
        <div className={styles.loadingContainer}>
          <Skeleton rows={8} />
        </div>
      </div>
    );
  }

  if (docQuery.error || !header) {
    return (
      <div className={styles.page}>
        <div className={styles.formSection}>
          <h2>Errore di caricamento</h2>
          <p>Impossibile caricare il documento richiesto.</p>
          <Button size="sm" variant="secondary" onClick={() => navigate('/archivio')}>
            Torna all'Archivio
          </Button>
        </div>
      </div>
    );
  }

  // Row Manipulation Handlers
  const handleUpdateRowField = (
    index: number,
    field: keyof LocalRow,
    value: string | boolean,
  ) => {
    setRows((prev) =>
      prev.map((row, idx) => (idx === index ? { ...row, [field]: value } : row)),
    );
  };

  const handleAddRow = (isDescriptive = false) => {
    setRows((prev) => [
      ...prev,
      {
        IDDocRiga: 0,
        CodArticolo: '',
        Desc: '',
        Qta: isDescriptive ? '0' : '1',
        Udm: isDescriptive ? '' : 'PZ',
        PrezzoNetto: '0',
        Sconti: '',
        isDescriptive,
      },
    ]);
  };

  const handleDeleteRow = (index: number) => {
    setRows((prev) => prev.filter((_, idx) => idx !== index));
  };

  const handleMoveRow = (index: number, direction: 'up' | 'down') => {
    const nextIndex = direction === 'up' ? index - 1 : index + 1;
    if (nextIndex < 0 || nextIndex >= rows.length) return;

    setRows((prev) => {
      const copy = [...prev];
      const temp = copy[index];
      if (!temp) return prev;
      copy[index] = copy[nextIndex]!;
      copy[nextIndex] = temp;
      return copy;
    });
  };

  const getRowNetTotal = (row: LocalRow) => {
    if (row.isDescriptive) return 0;
    const prezzoNetto = parseDecimal(row.PrezzoNetto) ?? 0;
    const qta = parseDecimal(row.Qta) ?? 0;
    let net = prezzoNetto * qta;
    if (!row.Sconti) return net;
    const discounts = row.Sconti
      .split('+')
      .map((d) => parseFloat(d))
      .filter((d) => !isNaN(d));
    for (const pct of discounts) {
      net = net * (1 - pct / 100);
    }
    return net;
  };

  // Calculate frontend approximate totals
  const totalNetto = rows.reduce((sum, row) => sum + getRowNetTotal(row), 0);
  const totalAcquisto = 0; // Not edited in this simple UI
  const totalGuadagno = totalNetto - totalAcquisto;

  const handleHeaderChange = (field: keyof DocumentState, value: string) => {
    setHeader((prev) => (prev ? { ...prev, [field]: value } : null));
  };

  const handleSave = () => {
    if (!header.Anagr_Nome) {
      alert("Il nome cliente/ragione sociale è obbligatorio!");
      return;
    }

    const payload: UpdateDocumentPayload = {
      ...header,
      Rows: rows.map((r) => ({
        IDDocRiga: r.IDDocRiga,
        CodArticolo: r.isDescriptive ? null : r.CodArticolo || null,
        Desc: r.Desc || null,
        Qta: r.isDescriptive ? null : normalizeDecimalInput(r.Qta),
        Udm: r.isDescriptive ? null : r.Udm || null,
        PrezzoNetto: r.isDescriptive ? null : normalizeDecimalInput(r.PrezzoNetto),
        Sconti: r.isDescriptive ? null : r.Sconti || null,
      })),
    };

    updateMutation.mutate(
      { idDoc, payload },
      {
        onSuccess: () => {
          navigate('/archivio');
        },
        onError: (err) => {
          alert(`Errore durante il salvataggio: ${err instanceof Error ? err.message : 'Errore sconosciuto'}`);
        },
      },
    );
  };

  return (
    <main className={styles.page}>
      <div className={styles.topBar}>
        <div className={styles.topBarLeft}>
          <button className={styles.backBtn} onClick={() => navigate('/archivio')}>
            <Icon name="arrow-left" size={16} /> Torna a Lista
          </button>
          <div className={styles.topBarTitle}>
            <h1>Modifica Preventivo {header.NumDoc}</h1>
          </div>
        </div>

        <div className={styles.topBarRight}>
          {/* Il PDF riflette i dati salvati, non le modifiche in corso. */}
          <ScaricaPdfButton
            doc={{
              IDDoc: idDoc,
              NumDoc: header.NumDoc || null,
              DataDoc: header.DataDoc || null,
              Anagr_Nome: header.Anagr_Nome || null,
            }}
          />
          <Button onClick={handleSave} disabled={updateMutation.isPending}>
            {updateMutation.isPending ? 'Salvataggio...' : 'Salva Documento'}
          </Button>
        </div>
      </div>

      <div className={styles.wizardLayout}>
        <div className={styles.summaryBanner}>
          <div className={styles.summaryBannerText}>
            <span className={styles.summaryBannerTitle}>{header.Anagr_Nome || 'Nessun cliente'}</span>
            <span className={styles.summaryBannerSubtitle}>
              {header.Anagr_Citta && `${header.Anagr_Citta} (${header.Anagr_Prov})`} | Pagamento: {header.Pagamento || '-'}
            </span>
          </div>
          <Button size="sm" variant="secondary" onClick={() => setDrawerOpen(true)}>
            <Icon name="settings" size={16} /> Modifica Dati Testata
          </Button>
        </div>

        <RowsEditor
          rows={rows}
          onUpdateRow={handleUpdateRowField}
          onAddRow={handleAddRow}
          onDeleteRow={handleDeleteRow}
          onMoveRow={handleMoveRow}
          getRowNetTotal={getRowNetTotal}
          expandedIndex={expandedDescIndex}
          onToggleExpand={(index) => setExpandedDescIndex(expandedDescIndex === index ? null : index)}
        />
        <TotalsSummary
          netto={totalNetto}
          acquisto={totalAcquisto}
          guadagno={totalGuadagno}
        />

        <Drawer
          open={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          size="xl"
          title="Dati Testata Documento"
        >
          <div style={{ padding: 'var(--space-5) var(--space-6)' }}>
            <HeaderForm header={header} onChange={handleHeaderChange} />
            <div style={{ marginTop: 'var(--space-5)', display: 'flex', justifyContent: 'flex-end' }}>
              <Button onClick={() => setDrawerOpen(false)}>Conferma</Button>
            </div>
          </div>
        </Drawer>
      </div>

    </main>
  );
}

function HeaderForm({
  header,
  onChange,
}: {
  header: DocumentState;
  onChange: (field: keyof DocumentState, value: string) => void;
}) {
  const pmQuery = usePaymentMethods();
  const [showAutocomplete, setShowAutocomplete] = useState(false);
  const [showModal, setShowModal] = useState(false);
  const [modalSearch, setModalSearch] = useState('');

  const paymentMethods = pmQuery.data ?? [];

  // Filter for autocomplete: match by name (case-insensitive)
  const filteredAutocomplete = useMemo(() => {
    const query = (header.Pagamento ?? '').toLowerCase();
    if (!query) return paymentMethods.slice(0, 8); // Show first 8 if empty
    return paymentMethods
      .filter((pm) => pm.nomePagamento.toLowerCase().includes(query))
      .slice(0, 8);
  }, [paymentMethods, header.Pagamento]);

  // Filter for modal
  const filteredModal = useMemo(() => {
    const query = modalSearch.toLowerCase();
    if (!query) return paymentMethods;
    return paymentMethods.filter(
      (pm) =>
        pm.nomePagamento.toLowerCase().includes(query) ||
        (pm.categPagamento?.toLowerCase().includes(query) ?? false)
    );
  }, [paymentMethods, modalSearch]);

  return (
    <div className={styles.formSection}>
      <h2>Dati Anagrafici & Testata</h2>
      <div className={styles.formField}>
        <span className={styles.formFieldLabel}>
          Cliente / Ragione Sociale <span className={styles.requiredDot} />
        </span>
        <input
          className={styles.formFieldInput}
          value={header.Anagr_Nome}
          onChange={(e) => onChange('Anagr_Nome', e.target.value)}
        />
      </div>

      <div className={styles.formGrid}>
        <div className={styles.formField}>
          <span className={styles.formFieldLabel}>Partita IVA</span>
          <input
            className={styles.formFieldInput}
            value={header.Anagr_PartitaIva}
            onChange={(e) => onChange('Anagr_PartitaIva', e.target.value)}
          />
        </div>
        <div className={styles.formField}>
          <span className={styles.formFieldLabel}>Codice Fiscale</span>
          <input
            className={styles.formFieldInput}
            value={header.Anagr_CodiceFiscale}
            onChange={(e) => onChange('Anagr_CodiceFiscale', e.target.value)}
          />
        </div>
      </div>

      <div className={styles.formField}>
        <span className={styles.formFieldLabel}>Indirizzo Fatturazione</span>
        <input
          className={styles.formFieldInput}
          value={header.Anagr_Indirizzo}
          onChange={(e) => onChange('Anagr_Indirizzo', e.target.value)}
        />
      </div>

      <div className={styles.formGrid}>
        <div className={styles.formField} style={{ flex: '0 0 80px' }}>
          <span className={styles.formFieldLabel}>CAP</span>
          <input
            className={styles.formFieldInput}
            value={header.Anagr_Cap}
            onChange={(e) => onChange('Anagr_Cap', e.target.value)}
          />
        </div>
        <div className={styles.formField}>
          <span className={styles.formFieldLabel}>Città</span>
          <input
            className={styles.formFieldInput}
            value={header.Anagr_Citta}
            onChange={(e) => onChange('Anagr_Citta', e.target.value)}
          />
        </div>
        <div className={styles.formField} style={{ flex: '0 0 60px' }}>
          <span className={styles.formFieldLabel}>Prov</span>
          <input
            className={styles.formFieldInput}
            value={header.Anagr_Prov}
            onChange={(e) => onChange('Anagr_Prov', e.target.value)}
          />
        </div>
      </div>

      <h2>Destinatario Diverso (Spedizione)</h2>
      <div className={styles.formField}>
        <span className={styles.formFieldLabel}>Nome Destinatario</span>
        <input
          className={styles.formFieldInput}
          value={header.Anagr_DestNome}
          onChange={(e) => onChange('Anagr_DestNome', e.target.value)}
          placeholder="Lasciare vuoto se uguale a fatturazione"
        />
      </div>

      <div className={styles.formField}>
        <span className={styles.formFieldLabel}>Indirizzo Spedizione</span>
        <input
          className={styles.formFieldInput}
          value={header.Anagr_DestIndirizzo}
          onChange={(e) => onChange('Anagr_DestIndirizzo', e.target.value)}
        />
      </div>

      <div className={styles.formGrid}>
        <div className={styles.formField} style={{ flex: '0 0 80px' }}>
          <span className={styles.formFieldLabel}>CAP</span>
          <input
            className={styles.formFieldInput}
            value={header.Anagr_DestCap}
            onChange={(e) => onChange('Anagr_DestCap', e.target.value)}
          />
        </div>
        <div className={styles.formField}>
          <span className={styles.formFieldLabel}>Città</span>
          <input
            className={styles.formFieldInput}
            value={header.Anagr_DestCitta}
            onChange={(e) => onChange('Anagr_DestCitta', e.target.value)}
          />
        </div>
        <div className={styles.formField} style={{ flex: '0 0 60px' }}>
          <span className={styles.formFieldLabel}>Prov</span>
          <input
            className={styles.formFieldInput}
            value={header.Anagr_DestProv}
            onChange={(e) => onChange('Anagr_DestProv', e.target.value)}
          />
        </div>
      </div>

      <h2>Pagamento & Altro</h2>
      <div className={styles.formField} style={{ position: 'relative' }}>
        <span className={styles.formFieldLabel}>Metodo Pagamento</span>
        <div className={styles.paymentInputWrapper}>
          <input
            className={styles.formFieldInput}
            value={header.Pagamento}
            onChange={(e) => {
              onChange('Pagamento', e.target.value);
              setShowAutocomplete(true);
            }}
            onFocus={() => setShowAutocomplete(true)}
            onBlur={() => {
              // Delay hide slightly so that click on option registers
              setTimeout(() => setShowAutocomplete(false), 200);
            }}
            placeholder="Seleziona o scrivi metodo..."
          />
          <button
            type="button"
            className={styles.paymentLookupBtn}
            onClick={() => {
              setModalSearch('');
              setShowModal(true);
            }}
            title="Apri ricerca avanzata"
          >
            <Icon name="search" size={16} />
          </button>
        </div>

        {/* Autocomplete Popover */}
        {showAutocomplete && filteredAutocomplete.length > 0 && (
          <div className={styles.autocompletePopover}>
            {filteredAutocomplete.map((pm) => (
              <button
                key={pm.nomePagamento}
                type="button"
                className={styles.autocompleteItem}
                onMouseDown={() => {
                  onChange('Pagamento', pm.nomePagamento);
                  setShowAutocomplete(false);
                }}
              >
                <div className={styles.autocompleteItemName}>{pm.nomePagamento}</div>
                {pm.categPagamento && (
                  <div className={styles.autocompleteItemCategory}>{pm.categPagamento}</div>
                )}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Search Modal Dialog */}
      {showModal && (
        <div className={styles.modalOverlay}>
          <div className={styles.modalContent}>
            <div className={styles.modalHeader}>
              <div className={styles.modalTitle}>
                <Icon name="credit-card" size={18} />
                <h3>Seleziona Metodo Pagamento</h3>
              </div>
              <button
                type="button"
                className={styles.modalCloseBtn}
                onClick={() => setShowModal(false)}
                aria-label="Chiudi"
              >
                <Icon name="x" size={16} />
              </button>
            </div>

            <div className={styles.modalBody}>
              <div className={styles.modalSearchWrapper}>
                <Icon name="search" size={16} className={styles.modalSearchIcon} />
                <input
                  type="text"
                  className={styles.modalSearchInput}
                  value={modalSearch}
                  onChange={(e) => setModalSearch(e.target.value)}
                  placeholder="Cerca metodo di pagamento o categoria..."
                  autoFocus
                />
              </div>

              <div className={styles.modalListWrap}>
                {pmQuery.isLoading ? (
                  <div className={styles.modalLoading}>Caricamento metodi di pagamento...</div>
                ) : filteredModal.length === 0 ? (
                  <div className={styles.modalEmpty}>Nessun metodo trovato per "{modalSearch}"</div>
                ) : (
                  <table className={styles.modalTable}>
                    <thead>
                      <tr>
                        <th>Nome Pagamento</th>
                        <th>Categoria</th>
                        <th>Dettagli / Rate</th>
                        <th>Azione</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredModal.map((pm) => {
                        // Parse rates into structured data
                        const parsedRates = (() => {
                          if (!pm.rate) return [];
                          const matches = pm.rate.match(/\[\d+\|[\d.]+\|[TF]\]/g);
                          if (!matches) return [];
                          return matches.map(m => {
                            const parts = m.slice(1, -1).split('|');
                            const daysStr = parts[0] ?? '0';
                            const percentStr = parts[1] ?? '0';
                            const endOfMonthVal = (parts[2] ?? 'F') === 'T';
                            return {
                              days: parseInt(daysStr, 10),
                              percent: parseFloat(percentStr),
                              endOfMonth: endOfMonthVal
                            };
                          });
                        })();

                        // Format category display label
                        const displayCategory = (() => {
                          if (!pm.categPagamento) return null;
                          const mappings: Record<string, string> = {
                            'AssegnoCirc': 'Assegno Circolare',
                            'Assegno': 'Assegno',
                            'Bonifico': 'Bonifico',
                            'Contanti': 'Contanti',
                            'RIBA': 'Ri.Ba.',
                            'Altro': 'Altro'
                          };
                          return mappings[pm.categPagamento] || pm.categPagamento;
                        })();

                        return (
                          <tr
                            key={pm.nomePagamento}
                            className={styles.modalTableRow}
                            onClick={() => {
                              onChange('Pagamento', pm.nomePagamento);
                              setShowModal(false);
                            }}
                          >
                            <td className={styles.modalCellName}>
                              <strong>{pm.nomePagamento}</strong>
                            </td>
                            <td>
                              {displayCategory ? (
                                <span className={styles.modalCategoryBadge}>{displayCategory}</span>
                              ) : (
                                '-'
                              )}
                            </td>
                            <td>
                              {parsedRates.length > 0 ? (
                                <div className={styles.ratesList}>
                                  {parsedRates.map((r, idx) => (
                                    <span key={idx} className={styles.rateBadge}>
                                      <span className={styles.rateDays}>
                                        {r.days === 0 ? 'Immediato' : `${r.days} gg`}
                                      </span>
                                      {r.endOfMonth && <span className={styles.rateFm}>FM</span>}
                                      {!(r.percent === 100 && parsedRates.length === 1) && (
                                        <span className={styles.ratePercent}>{r.percent}%</span>
                                      )}
                                    </span>
                                  ))}
                                </div>
                              ) : (
                                <span className={styles.modalCellRate}>{pm.rate || '-'}</span>
                              )}
                            </td>
                            <td className={styles.modalCellAction}>
                              <span className={styles.selectAction}>
                                Seleziona <Icon name="arrow-right" size={14} className={styles.selectArrow} />
                              </span>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      <div className={styles.formField}>
        <span className={styles.formFieldLabel}>Coordinate Bancarie</span>
        <input
          className={styles.formFieldInput}
          value={header.Pagam_CoordBancarie}
          onChange={(e) => onChange('Pagam_CoordBancarie', e.target.value)}
        />
      </div>

      <div className={styles.formField}>
        <span className={styles.formFieldLabel}>Data Documento</span>
        <input
          type="date"
          className={styles.formFieldInput}
          value={header.DataDoc}
          onChange={(e) => onChange('DataDoc', e.target.value)}
        />
      </div>

      <div className={styles.formField}>
        <span className={styles.formFieldLabel}>Descrizione Preventivo</span>
        <input
          className={styles.formFieldInput}
          value={header.DescDoc}
          onChange={(e) => onChange('DescDoc', e.target.value)}
        />
      </div>

      <div className={styles.formField}>
        <span className={styles.formFieldLabel}>Note Interne</span>
        <textarea
          className={styles.formFieldTextarea}
          value={header.NoteInterne}
          onChange={(e) => onChange('NoteInterne', e.target.value)}
        />
      </div>
    </div>
  );
}

// Sub-component for Rows Editor
function RowsEditor({
  rows,
  onUpdateRow,
  onAddRow,
  onDeleteRow,
  onMoveRow,
  getRowNetTotal,
  expandedIndex,
  onToggleExpand,
}: {
  rows: LocalRow[];
  onUpdateRow: (index: number, field: keyof LocalRow, value: string | boolean) => void;
  onAddRow: (isDescriptive?: boolean) => void;
  onDeleteRow: (index: number) => void;
  onMoveRow: (index: number, direction: 'up' | 'down') => void;
  getRowNetTotal: (row: LocalRow) => number;
  expandedIndex: number | null;
  onToggleExpand: (index: number) => void;
}) {
  return (
    <div className={styles.rowsContainer}>
      <div className={styles.rowsHeader}>
        <h2>Righe Documento</h2>
        <div className={styles.actionRow}>
          <Button size="sm" variant="secondary" onClick={() => onAddRow(false)}>
            <Icon name="shopping-cart" size={16} /> + Articolo
          </Button>
          <Button size="sm" variant="secondary" onClick={() => onAddRow(true)}>
            <Icon name="file-text" size={16} /> + Nota/Filler
          </Button>
        </div>
      </div>

      <div className={styles.editTableWrap}>
        <table className={styles.editTable}>
          <thead>
            <tr>
              <th style={{ width: '120px' }}>Codice</th>
              <th>Descrizione</th>
              <th style={{ width: '80px', textAlign: 'right' }}>Quantità</th>
              <th style={{ width: '60px' }}>U.M.</th>
              <th style={{ width: '100px', textAlign: 'right' }}>Prezzo Unit.</th>
              <th style={{ width: '80px', textAlign: 'right' }}>Sconti</th>
              <th style={{ width: '100px', textAlign: 'right' }}>Importo</th>
              <th style={{ width: '100px', textAlign: 'center' }}>Azioni</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row, index) => {
              const isExpanded = expandedIndex === index;
              const hasLongDesc = row.Desc.length > 50;

              return (
                <Fragment key={index}>
                  <tr className={row.isDescriptive ? styles.descriptiveRow : ''}>
                    {row.isDescriptive ? (
                      <>
                        <td style={{ color: 'var(--color-text-muted)', fontStyle: 'italic' }}>Nota / Separatore</td>
                        <td colSpan={5} className={styles.descriptiveCell}>
                          <div className={styles.descCellWrapper}>
                            <textarea
                              className={`${styles.cellInput} ${styles.descTextareaCompact}`}
                              value={row.Desc}
                              onChange={(e) => onUpdateRow(index, 'Desc', e.target.value)}
                              placeholder="Inserisci un testo per la nota o un separatore visivo..."
                              rows={1}
                            />
                            <button
                              type="button"
                              className={`${styles.expandBtn} ${isExpanded ? styles.expandBtnActive : ''}`}
                              onClick={() => onToggleExpand(index)}
                              title={isExpanded ? "Comprimi editor esteso" : "Espandi editor esteso"}
                            >
                              <Icon name={isExpanded ? "chevron-up" : "chevron-down"} size={14} />
                              {hasLongDesc && !isExpanded && (
                                <span className={styles.longDescIndicator} />
                              )}
                            </button>
                          </div>
                        </td>
                      </>
                    ) : (
                      <>
                        <td>
                          <input
                            className={styles.cellInput}
                            value={row.CodArticolo}
                            onChange={(e) => onUpdateRow(index, 'CodArticolo', e.target.value)}
                            placeholder="Cod. Articolo"
                          />
                        </td>
                        <td>
                          <div className={styles.descCellWrapper}>
                            <textarea
                              className={`${styles.cellInput} ${styles.descTextareaCompact}`}
                              value={row.Desc}
                              onChange={(e) => onUpdateRow(index, 'Desc', e.target.value)}
                              placeholder="Descrizione articolo..."
                              rows={1}
                            />
                            <button
                              type="button"
                              className={`${styles.expandBtn} ${isExpanded ? styles.expandBtnActive : ''}`}
                              onClick={() => onToggleExpand(index)}
                              title={isExpanded ? "Comprimi editor esteso" : "Espandi editor esteso"}
                            >
                              <Icon name={isExpanded ? "chevron-up" : "chevron-down"} size={14} />
                              {hasLongDesc && !isExpanded && (
                                <span className={styles.longDescIndicator} />
                              )}
                            </button>
                          </div>
                        </td>
                        <td>
                          <input
                            type="number"
                            step="0.0001"
                            inputMode="decimal"
                            className={`${styles.cellInput} ${styles.numCellInput}`}
                            value={row.Qta}
                            onChange={(e) => onUpdateRow(index, 'Qta', e.target.value)}
                          />
                        </td>
                        <td>
                          <input
                            className={styles.cellInput}
                            value={row.Udm}
                            onChange={(e) => onUpdateRow(index, 'Udm', e.target.value)}
                            placeholder="PZ"
                          />
                        </td>
                        <td>
                          <input
                            type="number"
                            step="0.0001"
                            inputMode="decimal"
                            className={`${styles.cellInput} ${styles.numCellInput}`}
                            value={row.PrezzoNetto}
                            onChange={(e) => onUpdateRow(index, 'PrezzoNetto', e.target.value)}
                          />
                        </td>
                        <td>
                          <input
                            className={styles.cellInput}
                            value={row.Sconti}
                            onChange={(e) => onUpdateRow(index, 'Sconti', e.target.value)}
                            placeholder="es. 10+5"
                          />
                        </td>
                      </>
                    )}
                    <td style={{ textAlign: 'right', fontWeight: 600 }}>
                      {row.isDescriptive ? '-' : formatMoney(getRowNetTotal(row))}
                    </td>
                    <td>
                      <div className={styles.actionCell}>
                        <button
                          type="button"
                          className={styles.rowBtn}
                          onClick={() => onMoveRow(index, 'up')}
                          disabled={index === 0}
                          title="Sposta Su"
                        >
                          ↑
                        </button>
                        <button
                          type="button"
                          className={styles.rowBtn}
                          onClick={() => onMoveRow(index, 'down')}
                          disabled={index === rows.length - 1}
                          title="Sposta Giù"
                        >
                          ↓
                        </button>
                        <button
                          type="button"
                          className={`${styles.rowBtn} ${styles.rowBtnDelete}`}
                          onClick={() => onDeleteRow(index)}
                          title="Rimuovi"
                        >
                          ×
                        </button>
                      </div>
                    </td>
                  </tr>

                  {isExpanded && (
                    <tr className={styles.expandedRow}>
                      <td colSpan={8}>
                        <div className={styles.expandedEditorContainer}>
                          <div className={styles.expandedEditorHeader}>
                            <div className={styles.expandedEditorTitle}>
                              <Icon name="file-text" size={14} />
                              <span>Descrizione Estesa & Specifiche Tecniche (Riga {index + 1})</span>
                            </div>
                            <div className={styles.expandedEditorInfo}>
                              <span>Workspace esteso. Supporta testo multi-riga.</span>
                            </div>
                          </div>
                          <textarea
                            className={styles.expandedTextarea}
                            value={row.Desc}
                            onChange={(e) => onUpdateRow(index, 'Desc', e.target.value)}
                            placeholder="Inserisci la descrizione dettagliata dell'articolo..."
                            autoFocus
                            rows={6}
                          />
                          <div className={styles.expandedEditorFooter}>
                            <Button
                              size="sm"
                              onClick={() => onToggleExpand(index)}
                            >
                              Conferma e Chiudi
                            </Button>
                          </div>
                        </div>
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// Sub-component for Totals Summary
function TotalsSummary({
  netto,
  acquisto,
  guadagno,
}: {
  netto: number;
  acquisto: number;
  guadagno: number;
}) {
  return (
    <div className={styles.totalsSection}>
      <div className={styles.totalRow}>
        <span className={styles.totalRowLabel}>Totale Imponibile:</span>
        <span className={styles.totalRowValue}>{formatMoney(netto)}</span>
      </div>
      <div className={styles.totalRow}>
        <span className={styles.totalRowLabel}>Stima Costo Acquisto:</span>
        <span className={styles.totalRowValue}>{formatMoney(acquisto)}</span>
      </div>
      <div className={styles.totalRow}>
        <span className={styles.totalRowLabel}>Stima Guadagno Margine:</span>
        <span className={`${styles.totalRowValue} ${styles.guadagnoValue}`}>{formatMoney(guadagno)}</span>
      </div>
      <div className={`${styles.totalRow} ${styles.grandTotalRow}`}>
        <span className={styles.totalRowLabel}>Totale Indicativo:</span>
        <span className={`${styles.totalRowValue} ${styles.grandTotalValue}`}>{formatMoney(netto)}</span>
      </div>
      <p style={{ fontSize: '0.7rem', color: 'var(--color-text-muted)', margin: 'var(--space-2) 0 0 0', lineHeight: 1.3 }}>
        * I totali precisi (inclusi calcoli IVA, tasse e sconti complessi) verranno ricalcolati dal server una volta salvato il documento.
      </p>
    </div>
  );
}
