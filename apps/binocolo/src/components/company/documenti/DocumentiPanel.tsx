import { Button, Icon, Skeleton } from '@mrsmith/ui';
import type { MACompanyDocument } from '../../../api/types';
import { TERMINAL_DRIVE_CODES, driveErrorCode, useCompanyDocumenti } from '../../../hooks/useCompanyDocumenti';
import styles from './documenti.module.css';

const FOLDER_MIME = 'application/vnd.google-apps.folder';

export function DocumentiPanel({
  companyKey,
  activeInitiativeId,
  hideHeading = false,
}: {
  companyKey: string;
  // Lens initiative: the backend resolves the card's BOUND subfolder id and the
  // highlight matches on that id — never on names, which can change freely on
  // both sides (PRD §3.3) without breaking the link or the pill.
  activeInitiativeId?: string;
  // When embedded inside a page that already owns the section title + landmark
  // (the Scheda blockHeader), the panel drops its own h3 and accessible name to
  // avoid a doubled heading and a nested duplicate landmark.
  hideHeading?: boolean;
}) {
  const query = useCompanyDocumenti(companyKey, activeInitiativeId);
  const docs = query.data;
  const items = docs?.items ?? [];

  // The error code string is what the frontend branches on (§9 of the PRD);
  // not_configured / trashed / outside_context_root are terminal, the rest are
  // retryable. The section always degrades locally — it never breaks the Scheda.
  const errorCode = query.isError ? driveErrorCode(query.error) : '';
  const terminalError = TERMINAL_DRIVE_CODES.has(errorCode);
  const showFolderLink = Boolean(docs && docs.folderWebViewLink);

  return (
    <section className={styles.panel} aria-label={hideHeading ? undefined : 'Documenti'}>
      {!hideHeading || showFolderLink ? (
        <div className={styles.heading}>
          {hideHeading ? null : (
            <div>
              <h3>Documenti</h3>
              <p>Cartella condivisa dell’azienda su Google Drive.</p>
            </div>
          )}
          {showFolderLink ? (
            <Button
              variant="secondary"
              size="sm"
              leftIcon={<Icon name="external-link" size={16} />}
              onClick={() => window.open(docs!.folderWebViewLink, '_blank', 'noopener')}
            >
              Apri cartella
            </Button>
          ) : null}
        </div>
      ) : null}

      {query.isLoading ? (
        <Skeleton rows={3} />
      ) : query.isError ? (
        <DocumentiError code={errorCode} terminal={terminalError} onRetry={() => void query.refetch()} />
      ) : items.length === 0 ? (
        <p className={styles.empty}>Nessun documento. I file caricati su Drive compariranno qui.</p>
      ) : (
        <ul className={styles.list}>
          {items.map((item) => (
            <DocumentRow key={item.id} item={item} active={Boolean(docs?.cardFolderId) && item.id === docs?.cardFolderId} />
          ))}
        </ul>
      )}
    </section>
  );
}

function DocumentRow({ item, active }: { item: MACompanyDocument; active: boolean }) {
  const isFolder = item.mimeType === FOLDER_MIME;
  return (
    <li>
      <a
        className={`${styles.item} ${active ? styles.itemActive : ''}`}
        href={item.webViewLink}
        target="_blank"
        rel="noopener noreferrer"
      >
        <span className={styles.itemIcon}>
          <Icon name={isFolder ? 'box' : 'file-text'} size={18} />
        </span>
        <span className={styles.itemBody}>
          <span className={styles.itemName}>{item.name || 'Senza nome'}</span>
          <span className={styles.itemMeta}>
            {isFolder ? 'Cartella' : 'File'}
            {item.modifiedAt ? ` · ${formatModified(item.modifiedAt)}` : ''}
          </span>
        </span>
        {active ? <span className={styles.lensPill}>Lente attiva</span> : null}
        <span className={styles.itemOpen}>
          <Icon name="external-link" size={14} />
        </span>
      </a>
    </li>
  );
}

function DocumentiError({ code, terminal, onRetry }: { code: string; terminal: boolean; onRetry: () => void }) {
  // Terminal states (config missing, trashed, moved folder) can't be fixed from
  // here, so they explain the situation without a retry. Generic/transient Drive
  // errors stay retryable.
  if (code === 'googledrive_not_configured') {
    return (
      <div className={styles.infoState} role="status">
        <span className={styles.stateIcon}><Icon name="info" size={18} /></span>
        <div>
          <p>Documenti non disponibili.</p>
          <span className={styles.stateHint}>Integrazione Google Drive non configurata.</span>
        </div>
      </div>
    );
  }
  if (code === 'googledrive_trashed') {
    return (
      <div className={styles.warnState} role="status">
        <span className={styles.stateIcon}><Icon name="triangle-alert" size={18} /></span>
        <div>
          <p>Cartella nel cestino.</p>
          <span className={styles.stateHint}>Ripristina la cartella azienda da Google Drive.</span>
        </div>
      </div>
    );
  }
  if (code === 'googledrive_outside_context_root') {
    return (
      <div className={styles.warnState} role="status">
        <span className={styles.stateIcon}><Icon name="triangle-alert" size={18} /></span>
        <div>
          <p>Cartella spostata.</p>
          <span className={styles.stateHint}>La cartella non si trova più sotto la radice configurata.</span>
        </div>
      </div>
    );
  }
  if (code === 'googledrive_not_found') {
    return (
      <div className={styles.warnState} role="status">
        <span className={styles.stateIcon}><Icon name="triangle-alert" size={18} /></span>
        <div>
          <p>Cartella non trovata o non accessibile.</p>
          <span className={styles.stateHint}>Eliminata da Google Drive o accesso revocato. Non viene ricreata automaticamente.</span>
        </div>
      </div>
    );
  }
  return (
    <div className={styles.errorState} role="alert">
      <span className={styles.stateIcon}><Icon name="triangle-alert" size={18} /></span>
      <div className={styles.errorBody}>
        <p>Documenti non disponibili.</p>
        {!terminal ? <Button size="sm" variant="secondary" onClick={onRetry}>Riprova</Button> : null}
      </div>
    </div>
  );
}

function formatModified(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return new Intl.DateTimeFormat('it-IT', { day: '2-digit', month: 'short', year: 'numeric' }).format(d);
}
