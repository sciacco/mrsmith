import { useEffect, useState } from 'react';
import { Button } from '@mrsmith/ui';
import type { OrderDetail, UpdateReferentsPayload } from '../api/types';
import styles from '../pages/OrderDetailPage.module.css';

interface ReferentiTabProps {
  order: OrderDetail;
  canEdit: boolean;
  saving: boolean;
  onSave: (payload: UpdateReferentsPayload) => void;
}

export function ReferentiTab({ order, canEdit, saving, onSave }: ReferentiTabProps) {
  const [form, setForm] = useState<UpdateReferentsPayload>(() => fromOrder(order));

  useEffect(() => {
    setForm(fromOrder(order));
  }, [order]);

  function update(key: keyof UpdateReferentsPayload, value: string) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  return (
    <section className={styles.cardSection}>
      <div className={styles.sectionHeader}>
        <h2>Referenti</h2>
        {!canEdit ? <span className={styles.readonlyPill}>Solo lettura</span> : null}
      </div>
      <div className={styles.referentGroups}>
        <ReferentGroup title="Tecnico" prefix="technical" form={form} disabled={!canEdit} onChange={update} />
        <ReferentGroup title="Altro tecnico" prefix="otherTechnical" form={form} disabled={!canEdit} onChange={update} />
        <ReferentGroup title="Amministrativo" prefix="admin" form={form} disabled={!canEdit} onChange={update} />
      </div>
      <div className={styles.actionRow}>
        <Button loading={saving} disabled={!canEdit} onClick={() => onSave(form)}>Salva referenti</Button>
      </div>
    </section>
  );
}

type Prefix = 'technical' | 'otherTechnical' | 'admin';

const keys: Record<Prefix, { name: keyof UpdateReferentsPayload; phone: keyof UpdateReferentsPayload; email: keyof UpdateReferentsPayload }> = {
  technical: { name: 'technical_name', phone: 'technical_phone', email: 'technical_email' },
  otherTechnical: { name: 'other_technical_name', phone: 'other_technical_phone', email: 'other_technical_email' },
  admin: { name: 'admin_name', phone: 'admin_phone', email: 'admin_email' },
};

function ReferentGroup({
  title,
  prefix,
  form,
  disabled,
  onChange,
}: {
  title: string;
  prefix: Prefix;
  form: UpdateReferentsPayload;
  disabled: boolean;
  onChange: (key: keyof UpdateReferentsPayload, value: string) => void;
}) {
  const group = keys[prefix];
  return (
    <div className={styles.referentGroup}>
      <h3>{title}</h3>
      <label className={styles.fieldLabel}>
        <span>Nome</span>
        <input className={styles.input} value={form[group.name]} disabled={disabled} onChange={(event) => onChange(group.name, event.target.value)} />
      </label>
      <label className={styles.fieldLabel}>
        <span>Telefono</span>
        <input className={styles.input} value={form[group.phone]} disabled={disabled} onChange={(event) => onChange(group.phone, event.target.value)} />
      </label>
      <label className={styles.fieldLabel}>
        <span>Email</span>
        <input className={styles.input} type="email" value={form[group.email]} disabled={disabled} onChange={(event) => onChange(group.email, event.target.value)} />
      </label>
    </div>
  );
}

function fromOrder(order: OrderDetail): UpdateReferentsPayload {
  return {
    technical_name: order.cdlan_rif_tech_nom ?? '',
    technical_phone: order.cdlan_rif_tech_tel ?? '',
    technical_email: order.cdlan_rif_tech_email ?? '',
    other_technical_name: order.cdlan_rif_altro_tech_nom ?? '',
    other_technical_phone: order.cdlan_rif_altro_tech_tel ?? '',
    other_technical_email: order.cdlan_rif_altro_tech_email ?? '',
    admin_name: order.cdlan_rif_adm_nom ?? '',
    admin_phone: order.cdlan_rif_adm_tech_tel ?? '',
    admin_email: order.cdlan_rif_adm_tech_email ?? '',
  };
}
