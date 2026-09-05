// Anagrafiche del catalogo (#158, §Catalogo 5): fornitori, aree di
// competenza (con collegamento al gruppo locale, necessario alle platee per
// area) e certificazioni, tutte su upsert sui path piatti della slice 6.1.
// I team si consultano qui in sola gestione minima: rinomina dei soli team
// non gestiti dalla sync, gli altri restano di sola lettura.

import { useState } from 'react';
import { Button, Icon, Modal, Skeleton, SingleSelect, StatusBadge, ToggleSwitch, VisuallyHidden, useToast } from '@mrsmith/ui';
import {
  useTrainingCertifications,
  useTrainingGroups,
  useTrainingSkillAreas,
  useTrainingTeams,
  useTrainingVendors,
  useUpsertCertification,
  useUpsertSkillArea,
  useUpsertTeam,
  useUpsertVendor,
} from '../../api/queries';
import type {
  CertificationCatalogRow,
  CertificationInput,
  SkillAreaInput,
  SkillAreaListRow,
  TeamInput,
  TeamListRow,
  VendorInput,
  VendorListRow,
} from '../../api/types';
import { LEVEL_OPTIONS } from '../../lib/levels';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import formStyles from '../requests/requestShared.module.css';
import listStyles from '../../pages/RequestsPage/listPage.module.css';
import styles from './catalog.module.css';

export function AnagraficheSection() {
  return (
    <div className={styles.section}>
      <VendorsTable />
      <SkillAreasTable />
      <CertificationsTable />
      <TeamsTable />
    </div>
  );
}

// ── Fornitori ──

function VendorsTable() {
  const vendors = useTrainingVendors();
  const [editing, setEditing] = useState<VendorListRow | 'create' | null>(null);

  return (
    <section className={styles.section}>
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionTitle}>Fornitori</h2>
        <Button variant="secondary" size="sm" leftIcon={<Icon name="plus" size={14} />} onClick={() => setEditing('create')}>
          Nuovo fornitore
        </Button>
      </div>
      {vendors.isLoading ? (
        <Skeleton rows={2} />
      ) : vendors.isError ? (
        <p className={listStyles.errorNotice}>Lettura dei fornitori non riuscita.</p>
      ) : (vendors.data ?? []).length === 0 ? (
        <p>Nessun fornitore registrato.</p>
      ) : (
        <div className={listStyles.tableWrap}>
          <table className={listStyles.table}>
            <thead>
              <tr>
                <th>Nome</th>
                <th>Sito</th>
                <th>Note</th>
                <th>Attivo</th>
                <th>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {(vendors.data ?? []).map((v) => (
                <tr key={v.id}>
                  <td>{v.name}</td>
                  <td>{v.website || '—'}</td>
                  <td>{v.notes || '—'}</td>
                  <td>{v.active ? 'Sì' : 'No'}</td>
                  <td>
                    <Button variant="ghost" size="sm" onClick={() => setEditing(v)}>
                      Modifica
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {editing && <VendorEditorModal vendor={editing === 'create' ? null : editing} onClose={() => setEditing(null)} />}
    </section>
  );
}

function VendorEditorModal({ vendor, onClose }: { vendor: VendorListRow | null; onClose: () => void }) {
  const { toast } = useToast();
  const upsert = useUpsertVendor();
  const [name, setName] = useState(vendor?.name ?? '');
  const [website, setWebsite] = useState(vendor?.website ?? '');
  const [notes, setNotes] = useState(vendor?.notes ?? '');
  const [active, setActive] = useState(vendor?.active ?? true);
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    if (name.trim() === '') return;
    setError(null);
    const input: VendorInput = { name: name.trim(), website: website.trim() || undefined, notes: notes.trim() || undefined, active };
    try {
      await upsert.mutateAsync({ id: vendor?.id, input });
      toast(vendor ? 'Fornitore aggiornato' : 'Fornitore creato');
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Salvataggio non riuscito'));
    }
  }

  return (
    <ModalShell title={vendor ? 'Modifica fornitore' : 'Nuovo fornitore'} onClose={onClose}>
      <RequiredField label="Nome" value={name} onChange={setName} />
      <label className={formStyles.field}>
        Sito web
        <input className={formStyles.input} value={website} onChange={(e) => setWebsite(e.target.value)} />
      </label>
      <label className={formStyles.field}>
        Note
        <textarea className={formStyles.textarea} value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
      </label>
      <div className={formStyles.field}>
        Attivo
        <ToggleSwitch id="vendor-active" checked={active} onChange={setActive} />
      </div>
      <ErrorPanel message={error} onDismiss={() => setError(null)} />
      <ModalActions onClose={onClose} pending={upsert.isPending} disabled={name.trim() === ''} onSubmit={submit} />
    </ModalShell>
  );
}

// ── Aree di competenza ──

function SkillAreasTable() {
  const skillAreas = useTrainingSkillAreas();
  const [editing, setEditing] = useState<SkillAreaListRow | 'create' | null>(null);

  return (
    <section className={styles.section}>
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionTitle}>Aree di competenza</h2>
        <Button variant="secondary" size="sm" leftIcon={<Icon name="plus" size={14} />} onClick={() => setEditing('create')}>
          Nuova area
        </Button>
      </div>
      {skillAreas.isLoading ? (
        <Skeleton rows={2} />
      ) : skillAreas.isError ? (
        <p className={listStyles.errorNotice}>Lettura delle aree non riuscita.</p>
      ) : (skillAreas.data ?? []).length === 0 ? (
        <p>Nessuna area di competenza registrata.</p>
      ) : (
        <div className={listStyles.tableWrap}>
          <table className={listStyles.table}>
            <thead>
              <tr>
                <th>Nome</th>
                <th>Codice</th>
                <th>Gruppo collegato</th>
                <th>Attiva</th>
                <th>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {(skillAreas.data ?? []).map((a) => (
                <tr key={a.id}>
                  <td>{a.name}</td>
                  <td>{a.code}</td>
                  <td>{a.customGroupName || '—'}</td>
                  <td>{a.active ? 'Sì' : 'No'}</td>
                  <td>
                    <Button variant="ghost" size="sm" onClick={() => setEditing(a)}>
                      Modifica
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {editing && <SkillAreaEditorModal area={editing === 'create' ? null : editing} onClose={() => setEditing(null)} />}
    </section>
  );
}

function SkillAreaEditorModal({ area, onClose }: { area: SkillAreaListRow | null; onClose: () => void }) {
  const { toast } = useToast();
  const groups = useTrainingGroups();
  const skillAreas = useTrainingSkillAreas();
  const upsert = useUpsertSkillArea();
  const [code, setCode] = useState(area?.code ?? '');
  const [name, setName] = useState(area?.name ?? '');
  const [description, setDescription] = useState(area?.description ?? '');
  const [customGroupId, setCustomGroupId] = useState(area?.customGroupId ?? '');
  const [parentId, setParentId] = useState(area?.parentId ?? '');
  const [active, setActive] = useState(area?.active ?? true);
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    if (code.trim() === '' || name.trim() === '') return;
    setError(null);
    const input: SkillAreaInput = {
      code: code.trim(),
      name: name.trim(),
      description: description.trim() || undefined,
      customGroupId: customGroupId || undefined,
      parentId: parentId || undefined,
      active,
    };
    try {
      await upsert.mutateAsync({ id: area?.id, input });
      toast(area ? 'Area aggiornata' : 'Area creata');
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Salvataggio non riuscito'));
    }
  }

  return (
    <ModalShell title={area ? 'Modifica area' : 'Nuova area'} onClose={onClose}>
      <div className={formStyles.row}>
        <RequiredField label="Codice" value={code} onChange={setCode} />
        <RequiredField label="Nome" value={name} onChange={setName} />
      </div>
      <label className={formStyles.field}>
        Gruppo locale collegato
        <SingleSelect
          options={(groups.data ?? []).map((g) => ({ value: g.id, label: g.name }))}
          selected={customGroupId || null}
          onChange={(v) => setCustomGroupId(v ?? '')}
          placeholder="Nessuno"
          allowClear
        />
        <span className={formStyles.hint}>Necessario perché l'area sia usabile come platea di una regola.</span>
      </label>
      <label className={formStyles.field}>
        Area padre
        <SingleSelect
          options={(skillAreas.data ?? []).filter((a) => a.id !== area?.id).map((a) => ({ value: a.id, label: a.name }))}
          selected={parentId || null}
          onChange={(v) => setParentId(v ?? '')}
          placeholder="Nessuna"
          allowClear
        />
      </label>
      <label className={formStyles.field}>
        Descrizione
        <textarea className={formStyles.textarea} value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
      </label>
      <div className={formStyles.field}>
        Attiva
        <ToggleSwitch id="skillarea-active" checked={active} onChange={setActive} />
      </div>
      <ErrorPanel message={error} onDismiss={() => setError(null)} />
      <ModalActions onClose={onClose} pending={upsert.isPending} disabled={code.trim() === '' || name.trim() === ''} onSubmit={submit} />
    </ModalShell>
  );
}

// ── Certificazioni ──

function CertificationsTable() {
  const certifications = useTrainingCertifications();
  const [editing, setEditing] = useState<CertificationCatalogRow | 'create' | null>(null);

  return (
    <section className={styles.section}>
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionTitle}>Certificazioni</h2>
        <Button variant="secondary" size="sm" leftIcon={<Icon name="plus" size={14} />} onClick={() => setEditing('create')}>
          Nuova certificazione
        </Button>
      </div>
      {certifications.isLoading ? (
        <Skeleton rows={2} />
      ) : certifications.isError ? (
        <p className={listStyles.errorNotice}>Lettura delle certificazioni non riuscita.</p>
      ) : (certifications.data ?? []).length === 0 ? (
        <p>Nessuna certificazione registrata.</p>
      ) : (
        <div className={listStyles.tableWrap}>
          <table className={listStyles.table}>
            <thead>
              <tr>
                <th>Nome</th>
                <th>Codice</th>
                <th>Emittente</th>
                <th>Area</th>
                <th>Validità tipica</th>
                <th>Attiva</th>
                <th>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {(certifications.data ?? []).map((c) => (
                <tr key={c.id}>
                  <td>{c.name}</td>
                  <td>{c.code}</td>
                  <td>{c.issuerVendorName || '—'}</td>
                  <td>{c.skillAreaName || '—'}</td>
                  <td>{c.typicalValidityMonths !== undefined ? `${c.typicalValidityMonths} mesi` : '—'}</td>
                  <td>{c.active ? 'Sì' : 'No'}</td>
                  <td>
                    <Button variant="ghost" size="sm" onClick={() => setEditing(c)}>
                      Modifica
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {editing && (
        <CertificationEditorModal certification={editing === 'create' ? null : editing} onClose={() => setEditing(null)} />
      )}
    </section>
  );
}

function CertificationEditorModal({
  certification,
  onClose,
}: {
  certification: CertificationCatalogRow | null;
  onClose: () => void;
}) {
  const { toast } = useToast();
  const vendors = useTrainingVendors();
  const skillAreas = useTrainingSkillAreas();
  const upsert = useUpsertCertification();
  const [code, setCode] = useState(certification?.code ?? '');
  const [name, setName] = useState(certification?.name ?? '');
  const [issuerVendorId, setIssuerVendorId] = useState(certification?.issuerVendorId ?? '');
  const [skillAreaId, setSkillAreaId] = useState(certification?.skillAreaId ?? '');
  const [typicalValidityMonths, setTypicalValidityMonths] = useState(
    certification?.typicalValidityMonths !== undefined ? String(certification.typicalValidityMonths) : '',
  );
  const [attestedLevel, setAttestedLevel] = useState(
    certification?.attestedLevel !== undefined ? String(certification.attestedLevel) : '',
  );
  const [description, setDescription] = useState(certification?.description ?? '');
  const [active, setActive] = useState(certification?.active ?? true);
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    if (code.trim() === '' || name.trim() === '') return;
    // Un livello attestato richiede l'area: non inviamo senza area né azzeriamo
    // silenziosamente il livello già scritto (il backend resta comunque autorevole).
    if (attestedLevel !== '' && skillAreaId === '') {
      setError('Indica l\'area di competenza per il livello attestato.');
      return;
    }
    setError(null);
    const input: CertificationInput = {
      code: code.trim(),
      name: name.trim(),
      issuerVendorId: issuerVendorId || undefined,
      skillAreaId: skillAreaId || undefined,
      typicalValidityMonths: typicalValidityMonths !== '' ? Number(typicalValidityMonths) : undefined,
      attestedLevel: attestedLevel !== '' ? Number(attestedLevel) : undefined,
      description: description.trim() || undefined,
      active,
    };
    try {
      await upsert.mutateAsync({ id: certification?.id, input });
      toast(certification ? 'Certificazione aggiornata' : 'Certificazione creata');
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Salvataggio non riuscito'));
    }
  }

  return (
    <ModalShell
      title={<span className={styles.certificationModalTitle}>{certification ? 'Modifica certificazione' : 'Nuova certificazione'}</span>}
      onClose={onClose}
      bodyModal
    >
      <div className={formStyles.row}>
        <RequiredField label="Codice" value={code} onChange={setCode} />
        <RequiredField label="Nome" value={name} onChange={setName} />
      </div>
      <div className={formStyles.row}>
        <label className={formStyles.field}>
          Emittente
          <SingleSelect
            options={(vendors.data ?? []).map((v) => ({ value: v.id, label: v.name }))}
            selected={issuerVendorId || null}
            onChange={(v) => setIssuerVendorId(v ?? '')}
            placeholder="Nessuno"
            allowClear
          />
        </label>
        <label className={formStyles.field}>
          Validità tipica (mesi)
          <input
            type="number"
            min={0}
            className={formStyles.input}
            value={typicalValidityMonths}
            onChange={(e) => setTypicalValidityMonths(e.target.value)}
          />
        </label>
      </div>
      <label className={formStyles.field}>
        Area di competenza
        <SingleSelect
          options={(skillAreas.data ?? []).map((a) => ({ value: a.id, label: a.name }))}
          selected={skillAreaId || null}
          onChange={(v) => setSkillAreaId(v ?? '')}
          placeholder="Nessuna"
          allowClear
        />
      </label>
      <label className={formStyles.field}>
        Livello attestato sull'area (0–5)
        <SingleSelect<number>
          options={LEVEL_OPTIONS}
          selected={attestedLevel !== '' ? Number(attestedLevel) : null}
          onChange={(v) => setAttestedLevel(v !== null ? String(v) : '')}
          placeholder="Nessuno"
          allowClear
          clearLabel="Nessuno"
        />
        <span className={formStyles.hint}>Un esame superato su questa certificazione genera la valutazione dell'area scelta.</span>
      </label>
      <label className={formStyles.field}>
        Descrizione
        <textarea className={formStyles.textarea} value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
      </label>
      <div className={formStyles.field}>
        Attiva
        <ToggleSwitch id="certification-active" checked={active} onChange={setActive} />
      </div>
      <ErrorPanel message={error} onDismiss={() => setError(null)} />
      <ModalActions onClose={onClose} pending={upsert.isPending} disabled={code.trim() === '' || name.trim() === ''} onSubmit={submit} />
    </ModalShell>
  );
}

// ── Team (gestione minima: rinomina dei soli non gestiti dalla sync) ──

function TeamsTable() {
  const teams = useTrainingTeams();
  const [editing, setEditing] = useState<TeamListRow | null>(null);

  return (
    <section className={styles.section}>
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionTitle}>Team</h2>
      </div>
      {teams.isLoading ? (
        <Skeleton rows={2} />
      ) : teams.isError ? (
        <p className={listStyles.errorNotice}>Lettura dei team non riuscita.</p>
      ) : (teams.data ?? []).length === 0 ? (
        <p>Nessun team registrato.</p>
      ) : (
        <div className={listStyles.tableWrap}>
          <table className={listStyles.table}>
            <thead>
              <tr>
                <th>Nome</th>
                <th>Codice</th>
                <th>Origine</th>
                <th>Lead</th>
                <th>Membri attivi</th>
                <th>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {(teams.data ?? []).map((t) => (
                <tr key={t.id}>
                  <td>{t.name}</td>
                  <td>{t.code}</td>
                  <td>
                    <StatusBadge
                      value={t.managedBySync ? 'synced' : 'local'}
                      label={t.managedBySync ? 'Da sync' : 'Locale'}
                      variant={t.managedBySync ? 'neutral' : 'accent'}
                    />
                  </td>
                  <td>{t.leads.length === 0 ? '—' : t.leads.map((l) => l.name).join(', ')}</td>
                  <td>{t.activeMembers}</td>
                  <td>
                    {!t.managedBySync && (
                      <Button variant="ghost" size="sm" onClick={() => setEditing(t)}>
                        Rinomina
                      </Button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {editing && <TeamRenameModal team={editing} onClose={() => setEditing(null)} />}
    </section>
  );
}

function TeamRenameModal({ team, onClose }: { team: TeamListRow; onClose: () => void }) {
  const { toast } = useToast();
  const upsert = useUpsertTeam();
  const [code, setCode] = useState(team.code);
  const [name, setName] = useState(team.name);
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    if (code.trim() === '' || name.trim() === '') return;
    setError(null);
    const input: TeamInput = { code: code.trim(), name: name.trim(), active: team.active };
    try {
      await upsert.mutateAsync({ id: team.id, input });
      toast('Team rinominato');
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Rinomina non riuscita'));
    }
  }

  return (
    <ModalShell title="Rinomina team" onClose={onClose}>
      <div className={formStyles.row}>
        <RequiredField label="Codice" value={code} onChange={setCode} />
        <RequiredField label="Nome" value={name} onChange={setName} />
      </div>
      <p className={formStyles.hint}>
        L'eventuale descrizione esistente del team non è leggibile qui: salvando viene azzerata.
      </p>
      <ErrorPanel message={error} onDismiss={() => setError(null)} />
      <ModalActions onClose={onClose} pending={upsert.isPending} disabled={code.trim() === '' || name.trim() === ''} onSubmit={submit} />
    </ModalShell>
  );
}

// ── Impalcatura comune dei piccoli editor di anagrafica ──

function RequiredField({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <label className={formStyles.field}>
      <span className={formStyles.labelHead}>
        {label}
        <span className={formStyles.requiredMarker} aria-hidden="true" />
        <VisuallyHidden>obbligatorio</VisuallyHidden>
      </span>
      <input className={formStyles.input} value={value} onChange={(e) => onChange(e.target.value)} />
    </label>
  );
}

function ModalActions({
  onClose,
  pending,
  disabled,
  onSubmit,
}: {
  onClose: () => void;
  pending: boolean;
  disabled: boolean;
  onSubmit: () => void;
}) {
  return (
    <div className={formStyles.actions}>
      <Button variant="ghost" size="md" onClick={onClose} disabled={pending}>
        Annulla
      </Button>
      <Button variant="primary" size="md" loading={pending} disabled={disabled} onClick={onSubmit}>
        Salva
      </Button>
    </div>
  );
}

function ModalShell({
  title,
  onClose,
  children,
  bodyModal = false,
}: {
  title: React.ReactNode;
  onClose: () => void;
  children: React.ReactNode;
  bodyModal?: boolean;
}) {
  return (
    <Modal open onClose={onClose} title={title} size="sm">
      <div className={bodyModal ? `${formStyles.body} ${formStyles.bodyModal}` : formStyles.body}>{children}</div>
    </Modal>
  );
}
