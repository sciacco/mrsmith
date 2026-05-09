import { Button, Icon, Modal, SearchInput, Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useState } from 'react';
import { useCameras, useCameraMutations, useGrappaDCIMMeta } from '../../api/queries';
import type { CameraInput, CameraItem } from '../../api/types';
import { ViewState } from '../../components/ViewState';
import styles from '../facilities/workspace.module.css';
import { TextField, errorText } from '../equipment/assetPageUtils';

export function CamerasPage() {
  const toast = useToast();
  const meta = useGrappaDCIMMeta();
  const canOperate = Boolean(meta.data?.canOperate);
  const [q, setQ] = useState('');
  const [editing, setEditing] = useState<CameraItem | null | 'new'>(null);
  const cameras = useCameras({ q });
  const mutations = useCameraMutations();
  async function saveCamera(input: CameraInput & { id?: number }) {
    if (input.ipaddr && !isValidIp(input.ipaddr)) {
      toast.toast('Indirizzo IP non valido.', 'error');
      return;
    }
    try {
      const result = await mutations.saveCamera.mutateAsync(input);
      toast.toast(result.message || 'Telecamera salvata.');
      setEditing(null);
    } catch (error) {
      toast.toast(errorText(error, 'Salvataggio telecamera non riuscito.'), 'error');
    }
  }
  return <section className={styles.page}><div className={styles.header}><div className={styles.titleBlock}><span className={styles.eyebrow}>Asset</span><h1 className={styles.title}>Telecamere</h1><p className={styles.subtitle}>Inventario telecamere con posizione e indirizzo IP validato.</p></div>{canOperate ? <Button onClick={() => setEditing('new')} leftIcon={<Icon name="plus" size={16} />}>Nuova telecamera</Button> : null}</div><div className={styles.toolbar}><SearchInput value={q} onChange={setQ} placeholder="Cerca telecamere..." /></div>{cameras.isLoading ? <div className={styles.panel}><Skeleton rows={8} /></div> : cameras.error ? <ViewState title="Telecamere non disponibili" message="Non e stato possibile caricare l'inventario telecamere." tone="error" /> : (cameras.data?.length ?? 0) === 0 ? <div className={styles.emptyPanel}><h3 className={styles.emptyTitle}>Nessuna telecamera trovata</h3><p className={styles.emptyText}>Modifica la ricerca o aggiungi una telecamera.</p></div> : <div className={styles.tableWrap}><table className={styles.table}><thead><tr><th>Codice</th><th>Modello</th><th>Posizione</th><th>IP</th><th>Stato</th><th>Azioni</th></tr></thead><tbody>{cameras.data?.map((item) => <tr key={item.id}><td><strong>{item.code}</strong><br /><span className={styles.muted}>{item.serial ?? '-'}</span></td><td>{item.brand} {item.model}</td><td>{item.position}</td><td>{item.ipaddr ?? '-'}</td><td><span className={styles.badgeMuted}>{item.status ?? '-'}</span></td><td>{canOperate ? <Button size="sm" variant="secondary" onClick={() => setEditing(item)}>Modifica</Button> : null}</td></tr>)}</tbody></table></div>}<CameraModal open={editing !== null} value={editing === 'new' ? null : editing} onClose={() => setEditing(null)} onSave={saveCamera} loading={mutations.saveCamera.isPending} /></section>;
}

function CameraModal({ open, value, onClose, onSave, loading }: { open: boolean; value: CameraItem | null; onClose: () => void; onSave: (value: CameraInput & { id?: number }) => void; loading: boolean }) {
  const [draft, setDraft] = useState<CameraInput>({ code: '', model: '', brand: '', position: '' });
  useEffect(() => setDraft(value ? { code: value.code, model: value.model, brand: value.brand, position: value.position, ipaddr: value.ipaddr, status: value.status, serial: value.serial } : { code: '', model: '', brand: '', position: '' }), [value, open]);
  return <Modal open={open} onClose={onClose} title={value ? 'Modifica telecamera' : 'Nuova telecamera'}><div className={styles.formGrid}><TextField label="Codice" value={draft.code} onChange={(code) => setDraft({ ...draft, code })} /><TextField label="Marca" value={draft.brand} onChange={(brand) => setDraft({ ...draft, brand })} /><TextField label="Modello" value={draft.model} onChange={(model) => setDraft({ ...draft, model })} /><TextField label="Posizione" value={draft.position} onChange={(position) => setDraft({ ...draft, position })} /><TextField label="IP" value={draft.ipaddr ?? ''} onChange={(ipaddr) => setDraft({ ...draft, ipaddr })} /><TextField label="Seriale" value={draft.serial ?? ''} onChange={(serial) => setDraft({ ...draft, serial })} /><TextField label="Stato" value={draft.status ?? ''} onChange={(status) => setDraft({ ...draft, status })} /></div>{draft.ipaddr && !isValidIp(draft.ipaddr) ? <p className={styles.emptyText}>Indirizzo IP non valido.</p> : null}<div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} disabled={!draft.code || !draft.model || !draft.brand || !draft.position || Boolean(draft.ipaddr && !isValidIp(draft.ipaddr))} onClick={() => onSave({ ...draft, id: value?.id })}>Salva</Button></div></Modal>;
}

function isValidIp(value: string) {
  return /^(\d{1,3}\.){3}\d{1,3}$/.test(value) && value.split('.').every((part) => Number(part) >= 0 && Number(part) <= 255);
}
