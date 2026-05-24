import { Button, Icon, Skeleton, useToast } from '@mrsmith/ui';
import { useMemo, useState } from 'react';
import { Navigate, useNavigate, useParams } from 'react-router-dom';
import type { SendToERPResponse, UpdateHeaderPayload, UpdateReferentsPayload } from '../api/types';
import {
  useActivateRow,
  useCustomers,
  useOrdiniDownloads,
  useOrder,
  useOrderRows,
  usePatchOrderHeader,
  usePatchReferents,
  usePatchSerialNumber,
  usePatchTechnicalNotes,
  useSendToERP,
  useTechnicalRows,
} from '../api/queries';
import {
  activationFormFilename,
  kickoffFilename,
  orderPdfFilename,
  signedPdfFilename,
} from '../api/pdf';
import { AziendaTab } from '../components/AziendaTab';
import { DetailHeader } from '../components/DetailHeader';
import { InfoTab } from '../components/InfoTab';
import { ReferentiTab } from '../components/ReferentiTab';
import { RigheTab } from '../components/RigheTab';
import { TechnicalNotesTab } from '../components/TechnicalNotesTab';
import { AltriDatiTab } from '../components/AltriDatiTab';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import { downloadBlob } from '../lib/downloads';
import { apiErrorMessage } from '../lib/errors';
import {
  canDownloadActivationFormPdf,
  canDownloadKickoffPdf,
  canDownloadOrderPdf,
  canDownloadSignedPdf,
  canEditBozzaHeader,
  canEditReferents,
  canShowArxivarFilePicker,
} from '../lib/permissions';
import styles from './OrderDetailPage.module.css';

type DetailTab = 'info' | 'azienda' | 'referenti' | 'righe' | 'tecnici' | 'altri';
type DownloadKind = 'kickoff' | 'activation' | 'order' | 'signed';

const tabLabels: Record<DetailTab, string> = {
  info: 'Info',
  azienda: 'Azienda',
  referenti: 'Referenti',
  righe: 'Righe',
  tecnici: 'Informazioni dai tecnici',
  altri: 'Altri dati',
};

export function OrderDetailPage() {
  const { id: rawId } = useParams();
  const id = rawId ? Number(rawId) : NaN;
  const navigate = useNavigate();
  const auth = useOptionalAuth();
  const roles = auth.user?.roles;
  const { toast } = useToast();
  const [activeTab, setActiveTab] = useState<DetailTab>('info');
  const [sendResult, setSendResult] = useState<SendToERPResponse | null>(null);
  const [savingSerialRow, setSavingSerialRow] = useState<number | null>(null);
  const [savingNotesRow, setSavingNotesRow] = useState<number | null>(null);
  const [downloading, setDownloading] = useState<DownloadKind | null>(null);

  const order = useOrder(Number.isFinite(id) ? id : null);
  const rows = useOrderRows(Number.isFinite(id) ? id : null);
  const technicalRows = useTechnicalRows(Number.isFinite(id) ? id : null);
  const canEditHeader = canEditBozzaHeader(order.data, roles);
  const customers = useCustomers(canEditHeader);
  const patchHeader = usePatchOrderHeader(id);
  const patchReferents = usePatchReferents(id);
  const patchSerial = usePatchSerialNumber(id);
  const patchNotes = usePatchTechnicalNotes(id);
  const activateRow = useActivateRow(id);
  const sendToERP = useSendToERP(id);
  const downloads = useOrdiniDownloads();

  const tabBadges = useMemo(
    () => ({ righe: rows.data?.length ?? 0, tecnici: technicalRows.data?.length ?? 0 }),
    [rows.data?.length, technicalRows.data?.length],
  );

  if (!Number.isFinite(id) || id <= 0) return <Navigate to="/ordini" replace />;

  if (order.isLoading) {
    return (
      <main className={styles.page}>
        <section className={styles.cardSection}><Skeleton rows={10} /></section>
      </main>
    );
  }

  if (order.error || !order.data) {
    return (
      <main className={styles.page}>
        <section className={styles.stateCard}>
          <div className={styles.errorIcon}><Icon name="triangle-alert" size={30} /></div>
          <h1>Ordine non disponibile</h1>
          <p>{apiErrorMessage(order.error, 'Non è stato possibile caricare il dettaglio ordine.')}</p>
          <Button variant="secondary" onClick={() => navigate('/ordini')}>Torna agli ordini</Button>
        </section>
      </main>
    );
  }

  const detail = order.data;
  const canEditRefs = canEditReferents(detail, roles);

  async function saveHeader(payload: UpdateHeaderPayload) {
    try {
      await patchHeader.mutateAsync(payload);
      toast('Dati conferma salvati');
    } catch (error) {
      toast(apiErrorMessage(error, 'Salvataggio non riuscito'), 'error');
    }
  }

  async function saveReferents(payload: UpdateReferentsPayload) {
    try {
      await patchReferents.mutateAsync(payload);
      toast('Referenti salvati');
    } catch (error) {
      toast(apiErrorMessage(error, 'Salvataggio referenti non riuscito'), 'error');
    }
  }

  async function saveSerial(rowId: number, serialNumber: string) {
    setSavingSerialRow(rowId);
    try {
      await patchSerial.mutateAsync({ rowId, serialNumber });
      toast('Seriale salvato');
    } catch (error) {
      toast(apiErrorMessage(error, 'Salvataggio seriale non riuscito'), 'error');
      throw error;
    } finally {
      setSavingSerialRow(null);
    }
  }

  async function saveNotes(rowId: number, technicalNotes: string) {
    setSavingNotesRow(rowId);
    try {
      await patchNotes.mutateAsync({ rowId, technicalNotes });
      toast('Note tecniche salvate');
    } catch (error) {
      toast(apiErrorMessage(error, 'Salvataggio note non riuscito'), 'error');
    } finally {
      setSavingNotesRow(null);
    }
  }

  async function confirmActivation(rowId: number, date: string) {
    try {
      await activateRow.mutateAsync({ rowId, activationDate: date });
      toast('Attivazione confermata');
    } catch (error) {
      toast(apiErrorMessage(error, 'Conferma attivazione non riuscita'), 'error');
      throw error;
    }
  }

  async function send(file: File) {
    try {
      const result = await sendToERP.mutateAsync(file);
      setSendResult(result);
      const hasErrors = result.rows.some((row) => row.status === 'error');
      if (!hasErrors && result.stateTransitioned && result.arxivarUploaded) {
        toast('Ordine inviato in ERP');
        navigate('/ordini');
        return;
      }
      if (result.warning) {
        toast(sendWarningToast(result.warning), 'warning');
      } else if (hasErrors) {
        toast('Invio parziale: verifica le righe segnalate', 'error');
      }
    } catch (error) {
      toast(apiErrorMessage(error, 'Invio in ERP non riuscito'), 'error');
    }
  }

  async function download(kind: DownloadKind) {
    setDownloading(kind);
    try {
      const blob = await ({
        kickoff: downloads.kickoff,
        activation: downloads.activationForm,
        order: downloads.orderPdf,
        signed: downloads.signedPdf,
      }[kind])(detail.id);
      const filename = {
        kickoff: kickoffFilename,
        activation: activationFormFilename,
        order: orderPdfFilename,
        signed: signedPdfFilename,
      }[kind](detail);
      downloadBlob(blob, filename);
    } catch (error) {
      toast(apiErrorMessage(error, 'Download non riuscito'), 'error');
    } finally {
      setDownloading(null);
    }
  }

  return (
    <main className={styles.page}>
      <DetailHeader
        order={detail}
        canKickoff={canDownloadKickoffPdf(detail, roles)}
        canActivationForm={canDownloadActivationFormPdf(detail, roles)}
        canOrderPdf={canDownloadOrderPdf(detail)}
        canSignedPdf={canDownloadSignedPdf(detail)}
        downloading={downloading}
        onBack={() => navigate('/ordini')}
        onDownload={(kind) => void download(kind)}
      />

      <section className={styles.detailSurface}>
        <div className={styles.tabs} role="tablist" aria-label="Sezioni dettaglio ordine">
          {(Object.keys(tabLabels) as DetailTab[]).map((tab) => (
            <button key={tab} type="button" role="tab" aria-selected={activeTab === tab} className={`${styles.tab} ${activeTab === tab ? styles.tabActive : ''}`} onClick={() => setActiveTab(tab)}>
              {tabLabels[tab]}
              {tab === 'righe' || tab === 'tecnici' ? <span className={styles.tabBadge}>{tabBadges[tab]}</span> : null}
            </button>
          ))}
        </div>
        <div className={styles.tabBody}>
          {activeTab === 'info' ? (
            <InfoTab
              order={detail}
              customers={customers.data ?? []}
              customersLoading={customers.isLoading}
              canEdit={canEditHeader}
              canUploadPdf={canShowArxivarFilePicker(detail, roles)}
              saving={patchHeader.isPending}
              sending={sendToERP.isPending}
              result={sendResult}
              onSaveHeader={(payload) => void saveHeader(payload)}
              onSendToErp={(file) => void send(file)}
            />
          ) : null}
          {activeTab === 'azienda' ? <AziendaTab order={detail} /> : null}
          {activeTab === 'referenti' ? <ReferentiTab order={detail} canEdit={canEditRefs} saving={patchReferents.isPending} onSave={(payload) => void saveReferents(payload)} /> : null}
          {activeTab === 'righe' ? (
            <RigheTab
              order={detail}
              rows={rows.data ?? []}
              loading={rows.isLoading}
              roles={roles}
              savingRowId={savingSerialRow}
              activationLoading={activateRow.isPending}
              onSaveSerial={(rowId, serialNumber) => saveSerial(rowId, serialNumber)}
              onActivate={(rowId, date) => confirmActivation(rowId, date)}
            />
          ) : null}
          {activeTab === 'tecnici' ? (
            <TechnicalNotesTab rows={technicalRows.data ?? []} loading={technicalRows.isLoading} savingRowId={savingNotesRow} onSaveNotes={(rowId, notes) => void saveNotes(rowId, notes)} />
          ) : null}
          {activeTab === 'altri' ? <AltriDatiTab order={detail} /> : null}
        </div>
      </section>
    </main>
  );
}

function sendWarningToast(code: string): string {
  switch (code) {
    case 'arxivar_upload_failed':
      return 'Ordine inviato, ma il documento firmato richiede una verifica.';
    default:
      return 'Ordine inviato, ma una verifica resta in sospeso.';
  }
}
