import { SingleSelect } from '@mrsmith/ui';
import type { CustomerRef } from '../api/types';

interface CustomerSelectProps {
  customers: CustomerRef[];
  value: number | null;
  currentName?: string | null;
  disabled?: boolean;
  onChange: (value: number | null) => void;
}

export function CustomerSelect({ customers, value, currentName, disabled, onChange }: CustomerSelectProps) {
  const options = customers.map((customer) => ({
    value: customer.id,
    label: customer.name,
    secondaryLabel: `ID cliente: ${customer.id}`,
  }));
  if (value != null && currentName && !options.some((option) => option.value === value)) {
    options.unshift({ value, label: currentName, secondaryLabel: `ID cliente: ${value}` });
  }

  return (
    <SingleSelect<number>
      options={options}
      selected={value}
      onChange={onChange}
      placeholder="Seleziona ragione sociale"
      allowClear
      clearLabel="Nessun cliente"
      disabled={disabled}
      searchable
    />
  );
}
