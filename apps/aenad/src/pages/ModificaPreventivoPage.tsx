import { Button, Icon, Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useState, useMemo } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  useQuoteDetails,
  useCreateQuote,
  useUpdateQuote,
  useQuoteReady,
  useQuoteCustomers,
  useCreateProspect,
  useQuoteArticles,
  useQuotePaymentMethods,
  useQuoteDefaults,
  usePdfExports,
  useCreatePdfExport,
  usePdfExportDownload,
  useAttachPdfExport,
  useQuoteStages,
  useTransitionStage,
  useHubSpotRetry,
} from '../api/queries';
import { formatMoney, calculateDiscountMultiplier } from '../utils/format';
import { downloadBlob } from '../utils/downloads';
import { useApiClient } from '../api/client';
import type {
  QuoteLineInput,
  CustomerSnapshotInput,
  ContactSnapshotInput,
  PaymentSnapshot,
  SaveQuotePayload,
  ArticleLineInitializer,
} from '../api/types';
import styles from './ModificaPreventivoPage.module.css';

// Regex to check if a search term is an email
const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function ModificaPreventivoPage() {
  const { id } = useParams<{ id: string }>();
  const isNew = !id || id === 'nuovo';
  const quoteId = isNew ? 0 : parseInt(id ?? '0', 10) || 0;
  const navigate = useNavigate();
  const { toast } = useToast();
  const api = useApiClient();

  // Queries
  const quoteQuery = useQuoteDetails(quoteId, quoteId > 0);
  const stagesQuery = useQuoteStages();
  const paymentMethodsQuery = useQuotePaymentMethods();
  const defaultsQuery = useQuoteDefaults();
  const pdfExportsQuery = usePdfExports(quoteId, quoteId > 0);

  // Mutations
  const createMutation = useCreateQuote();
  const updateMutation = useUpdateQuote();
  const readyMutation = useQuoteReady();
  const retrySyncMutation = useHubSpotRetry();
  const transitionStageMutation = useTransitionStage();
  const createPdfMutation = useCreatePdfExport();
  const attachPdfMutation = useAttachPdfExport();
  const createProspectMutation = useCreateProspect();
  const downloadPdf = usePdfExportDownload();

  // Tab State at Workspace level
  const [activeTab, setActiveTab] = useState<'edit' | 'preview' | 'logs' | 'pdf'>('edit');

  // Local Quote States
  const [hubspotCompanyId, setHubspotCompanyId] = useState<string | null>(null);
  const [hubspotContactId, setHubspotContactId] = useState<string | null>(null);
  const [hubspotPipelineId, setHubspotPipelineId] = useState<string | null>(null);
  const [hubspotPipelineLabel, setHubspotPipelineLabel] = useState<string | null>(null);
  const [hubspotDealstageId, setHubspotDealstageId] = useState<string | null>(null);
  const [hubspotDealstageLabel, setHubspotDealstageLabel] = useState<string | null>(null);

  const [customer, setCustomer] = useState<CustomerSnapshotInput>({
    name: '', vat: '', tax_code: '', pec: '', email: '',
    address: '', zip: '', city: '', province: '', country: 'IT',
    language: 'it', numero_azienda_snapshot: null
  });

  const [contact, setContact] = useState<ContactSnapshotInput>({
    first_name: '', last_name: '', full_name: '', email: '', role: ''
  });

  const [payment, setPayment] = useState<PaymentSnapshot>({
    method_code: '', method_label: '', bank_details: ''
  });

  const [documentDate, setDocumentDate] = useState(new Date().toISOString().slice(0, 10));
  const [description, setDescription] = useState('');
  const [internalNotes, setInternalNotes] = useState('');
  const [lines, setLines] = useState<QuoteLineInput[]>([]);
  const [originalAuthoringStatus, setOriginalAuthoringStatus] = useState<'draft' | 'ready' | 'archived' | null>(null);

  // Search autocompletes
  const [customerSearch, setCustomerSearch] = useState('');
  const [showCustomerDropdown, setShowCustomerDropdown] = useState(false);
  const [articleSearch, setArticleSearch] = useState('');
  const [activeRowIndex, setActiveRowIndex] = useState<number | null>(null);
  const [paymentSearch, setPaymentSearch] = useState('');
  const [showPaymentDropdown, setShowPaymentDropdown] = useState(false);

  // Prospect Form State
  const [showProspectForm, setShowProspectForm] = useState(false);
  const [prospectForm, setProspectForm] = useState({
    email: '', first_name: '', last_name: '', company_name: '', domain: ''
  });

  // CLI Command Line State
  const [cliCommand, setCliCommand] = useState('');

  // Search Queries Triggered on min characters
  const customersQuery = useQuoteCustomers(customerSearch, customerSearch.length >= 2);
  const articlesQuery = useQuoteArticles(articleSearch, articleSearch.length >= 2);

  // Sync loaded quote to local states
  useEffect(() => {
    if (quoteQuery.data) {
      const q = quoteQuery.data;
      setOriginalAuthoringStatus(q.authoring_status);
      setHubspotCompanyId(q.hubspot_company_id);
      setHubspotContactId(q.hubspot_contact_id);
      setHubspotPipelineId(q.hubspot_pipeline_id);
      setHubspotPipelineLabel(q.hubspot_pipeline_label);
      setHubspotDealstageId(q.hubspot_dealstage_id);
      setHubspotDealstageLabel(q.hubspot_dealstage_label);

      setCustomer({
        name: q.customer.name ?? '',
        vat: q.customer.vat ?? '',
        tax_code: q.customer.tax_code ?? '',
        pec: q.customer.pec ?? '',
        email: q.customer.email ?? '',
        address: q.customer.address ?? '',
        zip: q.customer.zip ?? '',
        city: q.customer.city ?? '',
        province: q.customer.province ?? '',
        country: q.customer.country ?? 'IT',
        language: q.customer.language ?? 'it',
        numero_azienda_snapshot: q.customer.numero_azienda_snapshot ?? null,
      });

      setContact({
        first_name: q.contact.first_name ?? '',
        last_name: q.contact.last_name ?? '',
        full_name: q.contact.full_name ?? '',
        email: q.contact.email ?? '',
        role: q.contact.role ?? '',
      });

      setPayment({
        method_code: q.payment.method_code ?? '',
        method_label: q.payment.method_label ?? '',
        bank_details: q.payment.bank_details ?? '',
      });

      setDocumentDate(q.document_date ? q.document_date.slice(0, 10) : new Date().toISOString().slice(0, 10));
      setDescription(q.description ?? '');
      setInternalNotes(q.internal_notes ?? '');

      setLines(
        q.lines.map((line) => ({
          position: line.position,
          line_type: line.line_type,
          item_code: line.item_code,
          item_description: line.item_description,
          description: line.description,
          unit_of_measure: line.unit_of_measure,
          qta: line.qta,
          unit_price: line.unit_price,
          discounts: line.discounts,
          cod_iva: line.cod_iva,
          iva_percent_snapshot: line.iva_percent_snapshot,
          purchase_unit_price: line.purchase_unit_price,
        }))
      );
    }
  }, [quoteQuery.data]);

  // Set default values for new quote
  useEffect(() => {
    if (isNew && defaultsQuery.data) {
      setDocumentDate(new Date().toISOString().slice(0, 10));
    }
  }, [isNew, defaultsQuery.data]);

  // Detect commercial/structural modifications for Demotion Warning
  const isCommercialModified = useMemo(() => {
    if (isNew) return false;
    if (!quoteQuery.data) return false;
    const init = quoteQuery.data;

    if (hubspotCompanyId !== init.hubspot_company_id) return true;
    if (customer.name !== init.customer.name) return true;
    if (customer.vat !== init.customer.vat) return true;
    if (customer.email !== init.customer.email) return true;
    if (customer.address !== init.customer.address) return true;

    if (payment.method_code !== init.payment.method_code) return true;
    if (payment.method_label !== init.payment.method_label) return true;

    if (documentDate !== (init.document_date ? init.document_date.slice(0, 10) : '')) return true;

    if (lines.length !== init.lines.length) return true;
    for (let i = 0; i < lines.length; i++) {
      const curr = lines[i]!;
      const orig = init.lines[i]!;
      if (curr.line_type !== orig.line_type) return true;
      if (curr.item_code !== orig.item_code) return true;
      if (curr.qta !== orig.qta) return true;
      if (curr.unit_price !== orig.unit_price) return true;
      if (curr.discounts !== orig.discounts) return true;
      if (curr.cod_iva !== orig.cod_iva) return true;
    }

    return false;
  }, [isNew, quoteQuery.data, hubspotCompanyId, customer, payment, documentDate, lines]);

  // Line Manipulation
  const handleUpdateLineField = (index: number, field: keyof QuoteLineInput, value: string | null) => {
    setLines((prev) =>
      prev.map((l, idx) => (idx === index ? { ...l, [field]: value } : l))
    );
  };

  const handleAddLine = (type: 'item' | 'description' | 'spacer' = 'item') => {
    setLines((prev) => [
      ...prev,
      {
        position: prev.length + 1,
        line_type: type,
        item_code: type === 'item' ? '' : null,
        item_description: null,
        description: type === 'description' ? 'Nuova nota descrittiva...' : null,
        unit_of_measure: type === 'item' ? 'PZ' : null,
        qta: type === 'item' ? '1' : null,
        unit_price: type === 'item' ? '0' : null,
        discounts: null,
        cod_iva: type === 'item' ? defaultsQuery.data?.line?.cod_iva ?? '22' : null,
        iva_percent_snapshot: null,
        purchase_unit_price: null,
      },
    ]);
  };

  const handleDeleteLine = (index: number) => {
    setLines((prev) =>
      prev.filter((_, idx) => idx !== index).map((l, idx) => ({ ...l, position: idx + 1 }))
    );
  };

  const handleMoveLine = (index: number, direction: 'up' | 'down') => {
    const swapIdx = direction === 'up' ? index - 1 : index + 1;
    if (swapIdx < 0 || swapIdx >= lines.length) return;

    setLines((prev) => {
      const next = [...prev];
      const temp = next[index]!;
      next[index] = next[swapIdx]!;
      next[swapIdx] = temp;
      return next.map((l, idx) => ({ ...l, position: idx + 1 }));
    });
  };

  // CLI parser
  const handleCLISubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const command = cliCommand.trim();
    if (!command) return;

    let parsed: QuoteLineInput;
    const defaultIva = defaultsQuery.data?.line?.cod_iva ?? '22';

    if (command.startsWith('//')) {
      parsed = {
        position: lines.length + 1,
        line_type: 'description',
        item_code: null,
        item_description: null,
        description: command.slice(2).trim(),
        unit_of_measure: null,
        qta: null,
        unit_price: null,
        discounts: null,
        cod_iva: null,
        iva_percent_snapshot: null,
        purchase_unit_price: null,
      };
    } else if (command === '---') {
      parsed = {
        position: lines.length + 1,
        line_type: 'spacer',
        item_code: null,
        item_description: null,
        description: null,
        unit_of_measure: null,
        qta: null,
        unit_price: null,
        discounts: null,
        cod_iva: null,
        iva_percent_snapshot: null,
        purchase_unit_price: null,
      };
    } else {
      const parts = command.split(/\s+/);
      const item_code = parts[0] || '';
      let qta = '1';
      let unit_price = '0';
      let discounts: string | null = null;
      let cod_iva = defaultIva;

      for (let i = 1; i < parts.length; i++) {
        const part = parts[i]!;
        if (part.includes('x')) {
          const sub = part.split('x');
          if (sub[0]) qta = sub[0];
          if (sub[1]) unit_price = sub[1];
        } else if (part.startsWith('-')) {
          discounts = part.slice(1);
        } else if (part.startsWith('#')) {
          cod_iva = part.slice(1);
        }
      }

      parsed = {
        position: lines.length + 1,
        line_type: 'item',
        item_code,
        item_description: null,
        description: null,
        unit_of_measure: 'PZ',
        qta,
        unit_price,
        discounts,
        cod_iva,
        iva_percent_snapshot: null,
        purchase_unit_price: null,
      };

      if (item_code) {
        try {
          const res = await api.get<ArticleLineInitializer[]>(
            `/aenad/v1/quotes/articles?q=${encodeURIComponent(item_code)}&limit=1`
          );
          if (res && res.length > 0 && res[0]) {
            parsed.item_description = res[0].item_description;
            parsed.description = res[0].description;
            parsed.unit_of_measure = res[0].unit_of_measure || 'PZ';
            if (unit_price === '0' && res[0].unit_price) {
              parsed.unit_price = res[0].unit_price;
            }
            parsed.cod_iva = res[0].cod_iva || parsed.cod_iva;
          }
        } catch (err) {
          console.error('Failed to resolve Alyante description', err);
        }
      }
    }

    setLines((prev) => [...prev, parsed]);
    setCliCommand('');
    toast('Riga aggiunta con successo.', 'success');
  };

  // Autocomplete Selectors
  const handleSelectCustomer = (selected: any) => {
    setHubspotCompanyId(selected.hubspot_company_id);
    setCustomer({
      name: selected.customer.name,
      vat: selected.customer.vat,
      tax_code: selected.customer.tax_code,
      pec: selected.customer.pec,
      email: selected.customer.email,
      address: selected.customer.address,
      zip: selected.customer.zip,
      city: selected.customer.city,
      province: selected.customer.province,
      country: selected.customer.country || 'IT',
      language: selected.customer.language || 'it',
      numero_azienda_snapshot: selected.customer.numero_azienda_snapshot,
    });
    setCustomerSearch(selected.customer.name || '');
    setShowCustomerDropdown(false);
    setShowProspectForm(false);
  };

  // Prospect Form Submission
  const handleCreateProspectSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!prospectForm.email || !prospectForm.company_name) {
      toast('Email e Nome Azienda sono obbligatori!', 'error');
      return;
    }

    createProspectMutation.mutate(
      {
        email: prospectForm.email,
        first_name: prospectForm.first_name || null,
        last_name: prospectForm.last_name || null,
        full_name:
          prospectForm.first_name && prospectForm.last_name
            ? `${prospectForm.first_name} ${prospectForm.last_name}`
            : null,
        role: null,
        company_name: prospectForm.company_name,
        domain: prospectForm.domain || null,
        customer: {
          name: prospectForm.company_name,
          vat: null,
          tax_code: null,
          pec: null,
          email: prospectForm.email,
          address: null,
          zip: null,
          city: null,
          province: null,
          country: 'IT',
          language: 'it',
          numero_azienda_snapshot: null,
        },
      },
      {
        onSuccess: (data) => {
          toast('Prospect HubSpot creato con successo!', 'success');
          setHubspotCompanyId(data.hubspot_company_id);
          setHubspotContactId(data.hubspot_contact_id);

          setCustomer({
            name: data.customer.name,
            vat: data.customer.vat,
            tax_code: data.customer.tax_code,
            pec: data.customer.pec,
            email: data.customer.email,
            address: data.customer.address,
            zip: data.customer.zip,
            city: data.customer.city,
            province: data.customer.province,
            country: data.customer.country || 'IT',
            language: data.customer.language || 'it',
            numero_azienda_snapshot: data.customer.numero_azienda_snapshot,
          });

          setContact({
            first_name: data.contact.first_name,
            last_name: data.contact.last_name,
            full_name: data.contact.full_name,
            email: data.contact.email,
            role: data.contact.role,
          });

          setShowProspectForm(false);
        },
        onError: (err) => {
          toast(
            `Errore creazione prospect: ${err instanceof Error ? err.message : 'Errore del server'}`,
            'error'
          );
        },
      }
    );
  };

  // Payment Method Selection
  const handleSelectPayment = (pm: any) => {
    setPayment({
      method_code: pm.cod_pagamento,
      method_label: pm.desc_pagamento,
      bank_details: '',
    });
    setPaymentSearch(pm.desc_pagamento);
    setShowPaymentDropdown(false);
  };

  // Article selection helper
  const handleSelectArticle = (index: number, article: ArticleLineInitializer) => {
    handleUpdateLineField(index, 'item_code', article.item_code);
    handleUpdateLineField(index, 'item_description', article.item_description);
    handleUpdateLineField(index, 'description', article.description);
    handleUpdateLineField(index, 'unit_of_measure', article.unit_of_measure || 'PZ');
    if (article.unit_price) {
      handleUpdateLineField(index, 'unit_price', article.unit_price);
    }
    handleUpdateLineField(index, 'cod_iva', article.cod_iva);
    setArticleSearch('');
    setActiveRowIndex(null);
  };

  // Local calculations for real-time preview
  const localTotals = useMemo(() => {
    let subtotalNet = 0;
    let subtotalVat = 0;

    const computedLines = lines.map((l) => {
      if (l.line_type !== 'item') {
        return { ...l, lineNet: 0, lineVat: 0 };
      }
      const q = parseFloat(l.qta ?? '0') || 0;
      const p = parseFloat(l.unit_price ?? '0') || 0;
      const mult = calculateDiscountMultiplier(l.discounts);
      const lineNet = q * p * mult;

      let vatRate = 22;
      if (l.cod_iva) {
        const digits = l.cod_iva.replace(/\D/g, '');
        if (digits) vatRate = parseInt(digits, 10);
      }
      const lineVat = lineNet * (vatRate / 100);

      subtotalNet += lineNet;
      subtotalVat += lineVat;

      return {
        ...l,
        lineNet,
        lineVat,
      };
    });

    const totalGross = subtotalNet + subtotalVat;

    return {
      lines: computedLines,
      subtotalNet,
      subtotalVat,
      totalGross,
    };
  }, [lines]);

  // Saving the document
  const handleSave = (onSuccessCallback?: (newId: number) => void) => {
    if (!customer.name) {
      toast('Il nome del cliente è obbligatorio!', 'error');
      return;
    }

    const payload: SaveQuotePayload = {
      hubspot_company_id: hubspotCompanyId,
      hubspot_contact_id: hubspotContactId,
      hubspot_pipeline_id: hubspotPipelineId,
      hubspot_pipeline_label: hubspotPipelineLabel,
      hubspot_dealstage_id: hubspotDealstageId,
      hubspot_dealstage_label: hubspotDealstageLabel,
      customer,
      contact,
      document_date: documentDate ? new Date(documentDate).toISOString() : null,
      payment_method_code: payment.method_code || null,
      payment_method_label: payment.method_label || null,
      payment_bank_details: payment.bank_details || null,
      description: description || null,
      internal_notes: internalNotes || null,
      lines: lines.map((l) => ({
        position: l.position,
        line_type: l.line_type,
        item_code: l.line_type === 'item' ? l.item_code || null : null,
        item_description: l.line_type === 'item' ? l.item_description || null : null,
        description: l.description || null,
        unit_of_measure: l.line_type === 'item' ? l.unit_of_measure || null : null,
        qta: l.line_type === 'item' ? l.qta || null : null,
        unit_price: l.line_type === 'item' ? l.unit_price || null : null,
        discounts: l.line_type === 'item' ? l.discounts || null : null,
        cod_iva: l.line_type === 'item' ? l.cod_iva || null : null,
        iva_percent_snapshot: l.iva_percent_snapshot || null,
        purchase_unit_price: l.purchase_unit_price || null,
      })),
    };

    if (isNew) {
      createMutation.mutate(payload, {
        onSuccess: (data) => {
          toast('Preventivo bozza creato con successo!', 'success');
          if (onSuccessCallback) {
            onSuccessCallback(data.id);
          } else {
            navigate(`/preventivi/${data.id}`);
          }
        },
        onError: (err) => {
          toast(
            `Errore durante la creazione: ${err instanceof Error ? err.message : 'Errore del server'}`,
            'error'
          );
        },
      });
    } else {
      updateMutation.mutate(
        { id: quoteId, payload },
        {
          onSuccess: (data) => {
            toast('Preventivo salvato con successo!', 'success');
            void quoteQuery.refetch();
            if (onSuccessCallback) {
              onSuccessCallback(data.id);
            }
          },
          onError: (err) => {
            toast(
              `Errore durante il salvataggio: ${err instanceof Error ? err.message : 'Errore del server'}`,
              'error'
            );
          },
        }
      );
    }
  };

  // Transition to Ready
  const handleMarkAsReady = () => {
    if (isNew) {
      handleSave((newId) => {
        readyMutation.mutate(newId, {
          onSuccess: () => {
            toast('Preventivo contrassegnato come Pronto!', 'success');
            navigate(`/preventivi/${newId}`);
          },
          onError: (err) => {
            toast(
              `Errore transizione ready: ${err instanceof Error ? err.message : 'Dati incompleti o non validi'}`,
              'error'
            );
          },
        });
      });
    } else {
      handleSave((savedId) => {
        readyMutation.mutate(savedId, {
          onSuccess: () => {
            toast('Preventivo contrassegnato come Pronto!', 'success');
            void quoteQuery.refetch();
          },
          onError: (err) => {
            toast(
              `Errore transizione ready: ${err instanceof Error ? err.message : 'Dati incompleti o non validi'}`,
              'error'
            );
          },
        });
      });
    }
  };

  // Retry HubSpot sync
  const handleRetrySync = () => {
    retrySyncMutation.mutate(quoteId, {
      onSuccess: () => {
        toast('Sincronizzazione HubSpot riaccodata!', 'success');
        void quoteQuery.refetch();
      },
      onError: (err) => {
        toast(`Errore di rinvio sync: ${err instanceof Error ? err.message : 'Errore del server'}`, 'error');
      },
    });
  };

  // Transition stage in logs tab
  const handleStageChange = (expectedId: string, targetId: string) => {
    transitionStageMutation.mutate(
      {
        quoteId,
        expectedDealstageId: expectedId,
        targetDealstageId: targetId,
      },
      {
        onSuccess: () => {
          toast('Stato HubSpot Deal aggiornato!', 'success');
          void quoteQuery.refetch();
        },
        onError: (err) => {
          toast(`Errore transizione stato: ${err instanceof Error ? err.message : 'Errore del server'}`, 'error');
        },
      }
    );
  };

  // PDF Export Handlers
  const handleGeneratePdf = () => {
    createPdfMutation.mutate(quoteId, {
      onSuccess: () => {
        toast('Generazione revisione PDF avviata.', 'success');
        void pdfExportsQuery.refetch();
      },
      onError: (err) => {
        toast(`Errore di generazione PDF: ${err instanceof Error ? err.message : 'Errore sconosciuto'}`, 'error');
      },
    });
  };

  const handleDownloadPdfExport = async (exportId: number, filename: string) => {
    try {
      const blob = await downloadPdf(quoteId, exportId);
      downloadBlob(blob, filename);
      toast('Esportazione PDF scaricata con successo.', 'success');
    } catch {
      toast('Errore durante il download del file PDF.', 'error');
    }
  };

  const handleAttachPdf = (exportId: number) => {
    attachPdfMutation.mutate(
      { quoteId, exportId },
      {
        onSuccess: () => {
          toast('Invio allegato su HubSpot accodato!', 'success');
          void pdfExportsQuery.refetch();
        },
        onError: (err) => {
          toast(`Errore invio HubSpot: ${err instanceof Error ? err.message : 'Errore sconosciuto'}`, 'error');
        },
      }
    );
  };

  // Loading skeleton states
  if (!isNew && quoteQuery.isLoading) {
    return (
      <div className={styles.page}>
        <div className={styles.loadingContainer}>
          <Skeleton rows={10} />
        </div>
      </div>
    );
  }

  const filteredPaymentMethods = paymentMethodsQuery.data ?? [];

  return (
    <main className={styles.page}>
      {/* Top Bar Header */}
      <div className={styles.topBar}>
        <div className={styles.topBarLeft}>
          <button className={styles.backBtn} onClick={() => navigate('/preventivi')}>
            <Icon name="arrow-left" size={16} /> Torna ai Preventivi
          </button>
          <div className={styles.topBarTitle}>
            <h1>{isNew ? 'Nuovo Preventivo' : `Preventivo ${quoteQuery.data?.quote_number || `Bozza #${quoteId}`}`}</h1>
            {!isNew && originalAuthoringStatus && (
              <span
                className={`${styles.badge} ${
                  originalAuthoringStatus === 'ready' ? styles.badgeReady : styles.badgeDraft
                }`}
              >
                {originalAuthoringStatus === 'ready' ? 'Pronto' : 'Bozza'}
              </span>
            )}
          </div>
        </div>

        <div className={styles.topBarRight}>
          {originalAuthoringStatus === 'draft' && (
            <Button size="md" variant="secondary" onClick={handleMarkAsReady}>
              <Icon name="check" size={16} /> Segna come Pronto
            </Button>
          )}
          <Button onClick={() => handleSave()} disabled={createMutation.isPending || updateMutation.isPending}>
            <Icon name="file-check" size={16} /> Salva Preventivo
          </Button>
        </div>
      </div>

      {/* Demotion Warning Banner */}
      {!isNew && originalAuthoringStatus === 'ready' && isCommercialModified && (
        <div className={styles.demotionBanner}>
          <Icon name="triangle-alert" size={16} />
          <span>Le modifiche a dati commerciali riporteranno questo preventivo in stato BOZZA al salvataggio.</span>
        </div>
      )}

      {/* Main Single Column Workspace Layout */}
      <div className={styles.workspaceLayout}>
        {/* Tab Navigation at top of workspace */}
        <div className={styles.tabsNav}>
          <button
            className={`${styles.tabBtn} ${activeTab === 'edit' ? styles.tabBtnActive : ''}`}
            onClick={() => setActiveTab('edit')}
          >
            Compilazione
          </button>
          <button
            className={`${styles.tabBtn} ${activeTab === 'preview' ? styles.tabBtnActive : ''}`}
            onClick={() => setActiveTab('preview')}
          >
            Anteprima Documento
          </button>
          <button
            className={`${styles.tabBtn} ${activeTab === 'logs' ? styles.tabBtnActive : ''}`}
            onClick={() => setActiveTab('logs')}
            disabled={isNew}
          >
            HubSpot Sync Log
          </button>
          <button
            className={`${styles.tabBtn} ${activeTab === 'pdf' ? styles.tabBtnActive : ''}`}
            onClick={() => setActiveTab('pdf')}
            disabled={isNew}
          >
            Revisioni PDF
          </button>
        </div>

        {/* Tab 1: Compilazione (Full Width) */}
        {activeTab === 'edit' && (
          <div className={styles.leftPane}>
            {/* Customer Search Section */}
            <div className={styles.formSection}>
              <h3>Risoluzione Cliente & Contatto</h3>
              <div className={styles.clientSearchWrapper}>
                <span className={styles.formFieldLabel}>
                  Cerca Cliente (Rag. Sociale, P.IVA, Email...) <span className={styles.requiredDot}>*</span>
                </span>
                <div className={styles.paymentInputWrapper}>
                  <input
                    type="text"
                    className={styles.formFieldInput}
                    value={customerSearch}
                    onChange={(e) => {
                      setCustomerSearch(e.target.value);
                      setShowCustomerDropdown(true);
                    }}
                    onFocus={() => setShowCustomerDropdown(true)}
                    placeholder="Scrivi per cercare..."
                  />
                  {customerSearch.length > 0 && (
                    <button
                      type="button"
                      className={styles.rowBtn}
                      onClick={() => {
                        setCustomerSearch('');
                        setHubspotCompanyId(null);
                      }}
                    >
                      <Icon name="x" size={16} />
                    </button>
                  )}
                </div>

                {/* Customer dropdown results */}
                {showCustomerDropdown && (customerSearch.length >= 2 || customersQuery.data) && (
                  <div className={styles.autocompletePopover}>
                    {customersQuery.isLoading ? (
                      <div style={{ padding: '8px', fontSize: '0.75rem' }}>Ricerca in corso...</div>
                    ) : customersQuery.data && customersQuery.data.length > 0 ? (
                      customersQuery.data.map((c) => (
                        <button
                          key={c.hubspot_company_id}
                          type="button"
                          className={styles.autocompleteItem}
                          onClick={() => handleSelectCustomer(c)}
                        >
                          <span className={styles.autocompleteItemName}>
                            {c.customer.name}
                            {c.customer.numero_azienda_snapshot ? ` (${c.customer.numero_azienda_snapshot})` : ''}
                          </span>
                          <span className={styles.autocompleteItemMeta}>
                            {c.domain || 'Nessun dominio'} | {c.customer.city || 'Nessuna città'}{c.customer.province ? ` (${c.customer.province})` : ''}
                          </span>
                        </button>
                      ))
                    ) : (
                      <div style={{ padding: '8px' }}>
                        <p style={{ fontSize: '0.75rem', margin: '0 0 6px 0' }}>Nessun cliente trovato.</p>
                        {emailRegex.test(customerSearch) ? (
                          <Button
                            size="sm"
                            onClick={() => {
                              setProspectForm({
                                email: customerSearch,
                                first_name: '',
                                last_name: '',
                                company_name: '',
                                domain: customerSearch.split('@')[1] || '',
                              });
                              setShowProspectForm(true);
                              setShowCustomerDropdown(false);
                            }}
                          >
                            Crea Prospect con "{customerSearch}"
                          </Button>
                        ) : (
                          <p style={{ fontSize: '0.7rem', color: 'var(--color-text-muted)' }}>
                            Inserisci una mail valida per abilitare la creazione rapida Prospect.
                          </p>
                        )}
                      </div>
                    )}
                  </div>
                )}
              </div>

              {/* Prospect Form */}
              {showProspectForm && (
                <form onSubmit={handleCreateProspectSubmit} className={styles.prospectForm}>
                  <div className={styles.prospectFormTitle}>
                    <Icon name="user" size={16} /> Creazione Prospect HubSpot Live
                  </div>
                  <div className={styles.formGrid}>
                    <div className={styles.formField}>
                      <span className={styles.formFieldLabel}>Email <span className={styles.requiredDot}>*</span></span>
                      <input
                        type="email"
                        className={styles.formFieldInput}
                        value={prospectForm.email}
                        onChange={(e) => setProspectForm({ ...prospectForm, email: e.target.value })}
                        required
                      />
                    </div>
                    <div className={styles.formField}>
                      <span className={styles.formFieldLabel}>Nome Azienda <span className={styles.requiredDot}>*</span></span>
                      <input
                        type="text"
                        className={styles.formFieldInput}
                        value={prospectForm.company_name}
                        onChange={(e) => setProspectForm({ ...prospectForm, company_name: e.target.value })}
                        required
                      />
                    </div>
                    <div className={styles.formField}>
                      <span className={styles.formFieldLabel}>Sito Web/Domain</span>
                      <input
                        type="text"
                        className={styles.formFieldInput}
                        value={prospectForm.domain}
                        onChange={(e) => setProspectForm({ ...prospectForm, domain: e.target.value })}
                      />
                    </div>
                  </div>
                  <div className={styles.formGrid}>
                    <div className={styles.formField}>
                      <span className={styles.formFieldLabel}>Nome Referente</span>
                      <input
                        type="text"
                        className={styles.formFieldInput}
                        value={prospectForm.first_name}
                        onChange={(e) => setProspectForm({ ...prospectForm, first_name: e.target.value })}
                      />
                    </div>
                    <div className={styles.formField}>
                      <span className={styles.formFieldLabel}>Cognome Referente</span>
                      <input
                        type="text"
                        className={styles.formFieldInput}
                        value={prospectForm.last_name}
                        onChange={(e) => setProspectForm({ ...prospectForm, last_name: e.target.value })}
                      />
                    </div>
                  </div>
                  <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end', marginTop: '4px' }}>
                    <Button size="sm" variant="secondary" onClick={() => setShowProspectForm(false)}>
                      Annulla
                    </Button>
                    <Button size="sm" type="submit" disabled={createProspectMutation.isPending}>
                      {createProspectMutation.isPending ? 'Creazione...' : 'Salva su HubSpot'}
                    </Button>
                  </div>
                </form>
              )}

              {/* Customer summary card */}
              {customer.name && (
                <div style={{ background: 'var(--color-surface)', padding: '12px', borderRadius: '8px', fontSize: '0.8rem' }}>
                  <strong>{customer.name}</strong>
                  {customer.email && <div>Email: {customer.email}</div>}
                  {customer.address && (
                    <div>
                      Indirizzo: {customer.address} - {customer.zip} {customer.city} ({customer.province})
                    </div>
                  )}
                  {contact.full_name && (
                    <div style={{ marginTop: '6px', borderTop: '1px solid var(--color-border-subtle)', paddingTop: '6px' }}>
                      Referente: {contact.full_name} ({contact.email || 'nessuna mail'}) {contact.role && `- ${contact.role}`}
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* Document Details Section */}
            <div className={styles.formSection}>
              <h3>Dati Documento</h3>
              <div className={styles.formGrid}>
                <div className={styles.formField}>
                  <span className={styles.formFieldLabel}>Data Preventivo</span>
                  <input
                    type="date"
                    className={styles.formFieldInput}
                    value={documentDate}
                    onChange={(e) => setDocumentDate(e.target.value)}
                  />
                </div>
                <div className={styles.formField}>
                  <span className={styles.formFieldLabel}>Metodo Pagamento</span>
                  <div className={styles.paymentInputWrapper} style={{ position: 'relative' }}>
                    <input
                      type="text"
                      className={styles.formFieldInput}
                      value={paymentSearch || payment.method_label || ''}
                      onChange={(e) => {
                        setPaymentSearch(e.target.value);
                        setShowPaymentDropdown(true);
                        setPayment((prev) => ({ ...prev, method_label: e.target.value }));
                      }}
                      onFocus={() => setShowPaymentDropdown(true)}
                      placeholder="Seleziona pagamento..."
                    />
                    {showPaymentDropdown && filteredPaymentMethods.length > 0 && (
                      <div className={styles.autocompletePopover}>
                        {filteredPaymentMethods
                          .filter((pm) =>
                            pm.desc_pagamento.toLowerCase().includes(paymentSearch.toLowerCase())
                          )
                          .map((pm) => (
                            <button
                              key={pm.cod_pagamento}
                              type="button"
                              className={styles.autocompleteItem}
                              onClick={() => handleSelectPayment(pm)}
                            >
                              <span className={styles.autocompleteItemName}>{pm.desc_pagamento}</span>
                            </button>
                          ))}
                      </div>
                    )}
                  </div>
                </div>
              </div>
              <div className={styles.formField}>
                <span className={styles.formFieldLabel}>Oggetto / Descrizione Breve</span>
                <input
                  type="text"
                  className={styles.formFieldInput}
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="es. Offerta Cloud..."
                />
              </div>
              <div className={styles.formField}>
                <span className={styles.formFieldLabel}>Note Interne (Non stampate)</span>
                <textarea
                  className={styles.formFieldTextarea}
                  value={internalNotes}
                  onChange={(e) => setInternalNotes(e.target.value)}
                  placeholder="Note ad uso interno..."
                />
              </div>
            </div>

            {/* Righe Offerta Section (Full Width Table) */}
            <div className={styles.formSection}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <h3>Righe Offerta</h3>
                <div style={{ display: 'flex', gap: '6px' }}>
                  <Button size="sm" variant="secondary" onClick={() => handleAddLine('item')}>
                    + Riga Prodotto
                  </Button>
                  <Button size="sm" variant="secondary" onClick={() => handleAddLine('description')}>
                    + Nota Testo
                  </Button>
                  <Button size="sm" variant="secondary" onClick={() => handleAddLine('spacer')}>
                    + Separatore
                  </Button>
                </div>
              </div>

              <div className={styles.rowsTableWrap}>
                <table className={styles.rowsTable}>
                  <thead>
                    <tr>
                      <th style={{ width: '40px' }}>Pos</th>
                      <th style={{ width: '120px' }}>Codice</th>
                      <th>Descrizione</th>
                      <th style={{ width: '70px', textAlign: 'right' }}>Qta</th>
                      <th style={{ width: '60px' }}>U.M.</th>
                      <th style={{ width: '90px', textAlign: 'right' }}>Prezzo Unit.</th>
                      <th style={{ width: '70px', textAlign: 'right' }}>Sconti</th>
                      <th style={{ width: '60px' }}>IVA</th>
                      <th style={{ width: '80px', textAlign: 'right' }}>Totale</th>
                      <th style={{ width: '80px' }}></th>
                    </tr>
                  </thead>
                  <tbody>
                    {lines.map((l, index) => {
                      const localLine = localTotals.lines[index]!;
                      if (l.line_type === 'spacer') {
                        return (
                          <tr key={index} className={styles.spacerRow}>
                            <td colSpan={9} className={styles.spacerCell}>
                              --- Separatore Visivo ---
                            </td>
                            <td>
                              <div className={styles.rowActions}>
                                <button className={styles.rowBtn} onClick={() => handleMoveLine(index, 'up')}>
                                  <Icon name="chevron-up" size={12} />
                                </button>
                                <button className={styles.rowBtn} onClick={() => handleMoveLine(index, 'down')}>
                                  <Icon name="chevron-down" size={12} />
                                </button>
                                <button className={styles.rowBtnDelete} onClick={() => handleDeleteLine(index)}>
                                  <Icon name="trash" size={14} />
                                </button>
                              </div>
                            </td>
                          </tr>
                        );
                      }
                      if (l.line_type === 'description') {
                        return (
                          <tr key={index} className={styles.descRow}>
                            <td style={{ fontStyle: 'italic', color: 'var(--color-text-muted)' }}>{index + 1}</td>
                            <td colSpan={8}>
                              <input
                                type="text"
                                className={styles.cellInputCompact}
                                style={{ fontStyle: 'italic' }}
                                value={l.description || ''}
                                onChange={(e) => handleUpdateLineField(index, 'description', e.target.value)}
                                placeholder="Testo descrittivo o nota..."
                              />
                            </td>
                            <td>
                              <div className={styles.rowActions}>
                                <button className={styles.rowBtn} onClick={() => handleMoveLine(index, 'up')}>
                                  <Icon name="chevron-up" size={12} />
                                </button>
                                <button className={styles.rowBtn} onClick={() => handleMoveLine(index, 'down')}>
                                  <Icon name="chevron-down" size={12} />
                                </button>
                                <button className={styles.rowBtnDelete} onClick={() => handleDeleteLine(index)}>
                                  <Icon name="trash" size={14} />
                                </button>
                              </div>
                            </td>
                          </tr>
                        );
                      }

                      return (
                        <tr key={index}>
                          <td>{index + 1}</td>
                          <td style={{ position: 'relative' }}>
                            <input
                              type="text"
                              className={styles.cellInputCompact}
                              value={l.item_code || ''}
                              onChange={(e) => {
                                handleUpdateLineField(index, 'item_code', e.target.value);
                                setArticleSearch(e.target.value);
                              }}
                              onFocus={() => {
                                setArticleSearch(l.item_code || '');
                                setActiveRowIndex(index);
                              }}
                              onBlur={() => {
                                setActiveRowIndex(null);
                              }}
                              placeholder="Cod. Articolo"
                            />
                            {activeRowIndex === index && (
                              <div className={styles.autocompletePopover} style={{ minWidth: '320px' }}>
                                {articleSearch.length < 2 ? (
                                  <div style={{ padding: '8px', fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
                                    Digita almeno 2 caratteri per cercare nel database...
                                  </div>
                                ) : articlesQuery.isLoading ? (
                                  <div style={{ padding: '8px', fontSize: '0.75rem' }}>Ricerca articoli...</div>
                                ) : articlesQuery.data && articlesQuery.data.length > 0 ? (
                                  articlesQuery.data.map((art) => (
                                    <button
                                      key={art.item_code}
                                      type="button"
                                      className={styles.autocompleteItem}
                                      onMouseDown={(e) => {
                                        e.preventDefault();
                                        handleSelectArticle(index, art);
                                      }}
                                    >
                                      <span className={styles.autocompleteItemName}>{art.item_code}</span>
                                      <span className={styles.autocompleteItemMeta}>
                                        {art.item_description} | {formatMoney(art.unit_price)}
                                      </span>
                                    </button>
                                  ))
                                ) : (
                                  <div style={{ padding: '8px', fontSize: '0.75rem' }}>Nessun articolo trovato</div>
                                )}
                              </div>
                            )}
                          </td>
                          <td>
                            <input
                              type="text"
                              className={styles.cellInputCompact}
                              value={l.item_description || l.description || ''}
                              onChange={(e) => handleUpdateLineField(index, 'item_description', e.target.value)}
                              placeholder="Descrizione articolo..."
                            />
                          </td>
                          <td>
                            <input
                              type="text"
                              className={styles.cellInputCompact}
                              style={{ textAlign: 'right' }}
                              value={l.qta || '0'}
                              onChange={(e) => handleUpdateLineField(index, 'qta', e.target.value)}
                            />
                          </td>
                          <td>
                            <input
                              type="text"
                              className={styles.cellInputCompact}
                              value={l.unit_of_measure || ''}
                              onChange={(e) => handleUpdateLineField(index, 'unit_of_measure', e.target.value)}
                              placeholder="PZ"
                            />
                          </td>
                          <td>
                            <input
                              type="text"
                              className={styles.cellInputCompact}
                              style={{ textAlign: 'right' }}
                              value={l.unit_price || '0'}
                              onChange={(e) => handleUpdateLineField(index, 'unit_price', e.target.value)}
                            />
                          </td>
                          <td>
                            <input
                              type="text"
                              className={styles.cellInputCompact}
                              style={{ textAlign: 'right' }}
                              value={l.discounts || ''}
                              onChange={(e) => handleUpdateLineField(index, 'discounts', e.target.value)}
                              placeholder="es. 10+5"
                            />
                          </td>
                          <td>
                            <input
                              type="text"
                              className={styles.cellInputCompact}
                              value={l.cod_iva || ''}
                              onChange={(e) => handleUpdateLineField(index, 'cod_iva', e.target.value)}
                            />
                          </td>
                          <td style={{ textAlign: 'right', fontWeight: 600 }}>
                            {formatMoney(localLine.lineNet)}
                          </td>
                          <td>
                            <div className={styles.rowActions}>
                              <button className={styles.rowBtn} onClick={() => handleMoveLine(index, 'up')}>
                                <Icon name="chevron-up" size={12} />
                              </button>
                              <button className={styles.rowBtn} onClick={() => handleMoveLine(index, 'down')}>
                                <Icon name="chevron-down" size={12} />
                              </button>
                              <button className={styles.rowBtnDelete} onClick={() => handleDeleteLine(index)}>
                                <Icon name="trash" size={14} />
                              </button>
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>

              {/* CLI Command Input bar */}
              <form onSubmit={handleCLISubmit} className={styles.cliInputWrapper}>
                <span className={styles.cliPrompt}>$</span>
                <input
                  type="text"
                  className={styles.cliInput}
                  value={cliCommand}
                  onChange={(e) => setCliCommand(e.target.value)}
                  placeholder="CLI: es. ABC123 5x100 -10+5 #22 | // Nota testo | --- per separatore"
                />
                <span className={styles.cliHelp}>Invio per aggiungere</span>
              </form>
            </div>
          </div>
        )}

        {/* Tab 2: Stripe Digital Preview */}
        {activeTab === 'preview' && (
          <div className={styles.stripeInvoice}>
            <div className={styles.invoiceHeader}>
              <div className={styles.invoiceBrand}>
                <h2>{customer.name || 'Nome Cliente'}</h2>
                <p>{customer.email || 'Nessun indirizzo email'}</p>
              </div>
              <div className={styles.invoiceMeta}>
                <span className={styles.invoiceMetaTitle}>PREVENTIVO DIGITAL</span>
                <div className={styles.invoiceMetaText}>
                  Data: {documentDate ? new Date(documentDate).toLocaleDateString('it-IT') : '-'}
                </div>
              </div>
            </div>

            <div className={styles.invoiceDetails}>
              <div className={styles.invoiceCol}>
                <h4>Destinatario Fattura</h4>
                <p>
                  <strong>{customer.name || '-'}</strong>
                  {customer.address && (
                    <>
                      <br />
                      {customer.address}
                      <br />
                      {customer.zip} {customer.city} ({customer.province})
                    </>
                  )}
                  {customer.vat && (
                    <>
                      <br />
                      P.IVA: {customer.vat}
                    </>
                  )}
                  {contact.full_name && (
                    <>
                      <br />
                      <span style={{ fontSize: '0.75rem', color: '#6b7280' }}>
                        C.A.: {contact.full_name} ({contact.email || 'nessuna mail'})
                      </span>
                    </>
                  )}
                </p>
              </div>
              <div className={styles.invoiceCol}>
                <h4>Metodo Pagamento</h4>
                <p>
                  {payment.method_label || 'Non selezionato'}
                  {payment.bank_details && (
                    <>
                      <br />
                      <span style={{ fontSize: '0.75rem', color: '#6b7280' }}>
                        Coordinate: {payment.bank_details}
                      </span>
                    </>
                  )}
                </p>
              </div>
            </div>

            {description && (
              <div style={{ marginBottom: 'var(--space-6)', fontSize: '0.875rem' }}>
                <strong>Oggetto:</strong> {description}
              </div>
            )}

            <table className={styles.invoiceTable}>
              <thead>
                <tr>
                  <th style={{ width: '60%' }}>Descrizione</th>
                  <th style={{ textAlign: 'right', width: '15%' }}>Qta</th>
                  <th style={{ textAlign: 'right', width: '25%' }}>Prezzo</th>
                </tr>
              </thead>
              <tbody>
                {localTotals.lines.map((l, idx) => {
                  if (l.line_type === 'spacer') {
                    return (
                      <tr key={idx}>
                        <td colSpan={3} style={{ borderBottom: '1px dashed #e5e7eb', height: '10px' }} />
                      </tr>
                    );
                  }
                  if (l.line_type === 'description') {
                    return (
                      <tr key={idx}>
                        <td colSpan={3} style={{ fontStyle: 'italic', color: '#6b7280', fontSize: '0.75rem' }}>
                          {l.description}
                        </td>
                      </tr>
                    );
                  }
                  return (
                    <tr key={idx}>
                      <td data-label="Descrizione">
                        <div className={styles.invoiceTableTextPrimary}>{l.item_description || l.item_code}</div>
                        {l.description && (
                          <div className={styles.invoiceTableTextSecondary}>{l.description}</div>
                        )}
                      </td>
                      <td data-label="Qta" style={{ textAlign: 'right' }}>
                        {l.qta} {l.unit_of_measure}
                      </td>
                      <td data-label="Prezzo" style={{ textAlign: 'right', fontWeight: 600 }}>
                        {formatMoney(l.lineNet)}
                        {l.discounts && (
                          <div style={{ fontSize: '0.7rem', color: '#ef4444' }}>Sconto: -{l.discounts}%</div>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>

            <div className={styles.invoiceTotals}>
              <div className={styles.invoiceTotalsRow}>
                <span>Imponibile</span>
                <span>{formatMoney(localTotals.subtotalNet)}</span>
              </div>
              <div className={styles.invoiceTotalsRow}>
                <span>IVA</span>
                <span>{formatMoney(localTotals.subtotalVat)}</span>
              </div>
              <div className={`${styles.invoiceTotalsRow} ${styles.invoiceTotalsGrand}`}>
                <span>Totale</span>
                <span>{formatMoney(localTotals.totalGross)}</span>
              </div>
            </div>
          </div>
        )}

        {/* Tab 3: HubSpot logs Timeline & stages */}
        {activeTab === 'logs' && quoteQuery.data && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
            <div className={styles.syncStatusCard}>
              <div className={styles.syncStatusHeader}>
                <div className={styles.syncStatusTitle}>Stato HubSpot Sync</div>
                <div>
                  {quoteQuery.data.hubspot_sync_status === 'pending' && (
                    <span className={`${styles.badge} ${styles.badgeHubSpotPending}`}>In Coda</span>
                  )}
                  {quoteQuery.data.hubspot_sync_status === 'succeeded' && (
                    <span className={`${styles.badge} ${styles.badgeHubSpotSucceeded}`}>Sincronizzato</span>
                  )}
                  {quoteQuery.data.hubspot_sync_status === 'failed' && (
                    <span className={`${styles.badge} ${styles.badgeHubSpotFailed}`}>Fallito</span>
                  )}
                </div>
              </div>
              {quoteQuery.data.hubspot_synced_at && (
                <div style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
                  Ultimo sync: {new Date(quoteQuery.data.hubspot_synced_at).toLocaleString('it-IT')}
                </div>
              )}
              {quoteQuery.data.hubspot_sync_error && (
                <div className={styles.syncErrorAlert}>
                  <strong>Dettaglio errore:</strong> {quoteQuery.data.hubspot_sync_error}
                </div>
              )}
              {quoteQuery.data.hubspot_sync_status === 'failed' && (
                <Button size="sm" onClick={handleRetrySync} disabled={retrySyncMutation.isPending}>
                  <Icon name="loader" size={14} /> Riprova Sincronizzazione Ora
                </Button>
              )}
            </div>

            {stagesQuery.data && (
              <div className={styles.syncStatusCard}>
                <div className={styles.syncStatusTitle}>HubSpot Deal Stage Timeline</div>
                <div style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
                  Clicca su uno stato per effettuare una transizione del Deal.
                </div>
                <div className={styles.stageSelectorWrapper}>
                  <div className={styles.stagesTimeline}>
                    {stagesQuery.data.items.map((st: any) => {
                      const currentStageItem = stagesQuery.data.items.find(
                        (item: any) => item.id === quoteQuery.data?.hubspot_dealstage_id
                      );
                      const isCurrent = st.id === quoteQuery.data?.hubspot_dealstage_id;
                      const isActive =
                        st.display_order != null &&
                        currentStageItem?.display_order != null &&
                        st.display_order <= currentStageItem.display_order;

                      return (
                        <div
                          key={st.id}
                          className={`${styles.stageTimelineNode} ${
                            isCurrent
                              ? styles.stageTimelineNodeCurrent
                              : isActive
                              ? styles.stageTimelineNodeActive
                              : ''
                          }`}
                          onClick={() => {
                            if (quoteQuery.data?.hubspot_dealstage_id && st.id !== quoteQuery.data.hubspot_dealstage_id) {
                              handleStageChange(quoteQuery.data.hubspot_dealstage_id || '', st.id);
                            }
                          }}
                          title={`Sposta in: ${st.label}`}
                        >
                          <div className={styles.stageTimelineNodeLabel}>{st.label || st.id}</div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              </div>
            )}

            <div className={styles.logsTimeline}>
              {quoteQuery.data.events && quoteQuery.data.events.length > 0 ? (
                quoteQuery.data.events.map((evt: any) => {
                  const isFailed = evt.event_type.includes('failed') || evt.event_type.includes('error');
                  const isSuccess = evt.event_type.includes('success') || evt.event_type.includes('ready');

                  return (
                    <div key={evt.id} className={styles.logItem}>
                      <div
                        className={`${styles.logIcon} ${
                          isSuccess ? styles.logIconSuccess : isFailed ? styles.logIconFailed : ''
                        }`}
                      >
                        <Icon
                          name={isSuccess ? 'check-circle' : isFailed ? 'x-circle' : 'info'}
                          size={16}
                        />
                      </div>
                      <div className={styles.logContent}>
                        <div className={styles.logHeader}>
                          <div className={styles.logTitle}>{evt.event_type}</div>
                          <div className={styles.logTime}>
                            {new Date(evt.created_at).toLocaleString('it-IT')}
                          </div>
                        </div>
                        <div className={styles.logActor}>Operatore: {evt.actor_subject}</div>
                        {evt.payload && Object.keys(evt.payload).length > 0 && (
                          <pre className={styles.logPayload}>
                            {JSON.stringify(evt.payload, null, 2)}
                          </pre>
                        )}
                      </div>
                    </div>
                  );
                })
              ) : (
                <div style={{ textAlign: 'center', padding: '16px', color: 'var(--color-text-muted)' }}>
                  Nessun evento registrato.
                </div>
              )}
            </div>
          </div>
        )}

        {/* Tab 4: PDF Export Panel */}
        {activeTab === 'pdf' && (
          <div className={styles.pdfExportsCard}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div className={styles.syncStatusTitle}>Revisioni ed Esportazioni PDF</div>
              <Button size="sm" onClick={handleGeneratePdf} disabled={createPdfMutation.isPending}>
                Genera Nuova Revisione
              </Button>
            </div>

            <div className={styles.pdfExportsList}>
              {pdfExportsQuery.data && pdfExportsQuery.data.length > 0 ? (
                pdfExportsQuery.data.map((exportItem: any) => {
                  const hsAttached = exportItem.hubspot_attachment_status === 'succeeded';
                  const hsFailed = exportItem.hubspot_attachment_status === 'failed';
                  const hsPending = exportItem.hubspot_attachment_status === 'pending';

                  return (
                    <div key={exportItem.id} className={styles.pdfExportItem}>
                      <div className={styles.pdfExportMeta}>
                        <span className={styles.pdfExportTitle}>Revisione N. {exportItem.revision}</span>
                        <span className={styles.pdfExportDate}>
                          Data: {new Date(exportItem.created_at).toLocaleString('it-IT')}
                        </span>
                        {exportItem.hubspot_attachment_status && (
                          <span style={{ fontSize: '0.725rem', display: 'flex', alignItems: 'center', gap: '4px' }}>
                            HubSpot:
                            {hsAttached && (
                              <span style={{ color: 'var(--color-success)', fontWeight: 700 }}>Inviato</span>
                            )}
                            {hsFailed && (
                              <span style={{ color: 'var(--color-error)', fontWeight: 700 }}>Invio Fallito</span>
                            )}
                            {hsPending && (
                              <span style={{ color: 'var(--color-warning-strong)', fontWeight: 700 }}>In Coda</span>
                            )}
                          </span>
                        )}
                      </div>

                      <div className={styles.pdfExportActions}>
                        <button
                          className={styles.rowBtn}
                          onClick={() => handleDownloadPdfExport(exportItem.id, exportItem.filename)}
                          title="Scarica PDF"
                        >
                          <Icon name="download" size={16} />
                        </button>
                        {!hsAttached && (
                          <button
                            className={styles.rowBtn}
                            onClick={() => handleAttachPdf(exportItem.id)}
                            disabled={attachPdfMutation.isPending}
                            title="Invia allegato a Deal HubSpot"
                          >
                            <Icon name="external-link" size={16} />
                          </button>
                        )}
                      </div>
                    </div>
                  );
                })
              ) : (
                <div style={{ textAlign: 'center', padding: '16px', color: 'var(--color-text-muted)' }}>
                  Nessuna revisione PDF generata. Clicca su "Genera Nuova Revisione".
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </main>
  );
}
