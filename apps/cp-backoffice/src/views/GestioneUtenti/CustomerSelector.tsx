import { SingleSelect } from '@mrsmith/ui';
import type { Customer } from '../../api/customers';

interface CustomerSelectorProps {
  customers: Customer[] | undefined;
  selectedId: number | null;
  onChange: (id: number | null) => void;
  loading: boolean;
  error: boolean;
}

// CustomerSelector wraps @mrsmith/ui SingleSelect so the caller only ever
// touches the business object (Customer) and an id. Placeholder and empty
// states are built-in.
export function CustomerSelector({
  customers,
  selectedId,
  onChange,
  loading,
  error,
}: CustomerSelectorProps) {
  if (loading) {
    return <div>Caricamento aziende...</div>;
  }

  if (error) {
    return <div>Elenco aziende non disponibile.</div>;
  }

  const options = (customers ?? []).map((c) => ({
    value: c.id,
    label: c.name,
  }));

  return (
    <SingleSelect
      options={options}
      selected={selectedId}
      onChange={(value) => onChange(value as number | null)}
      placeholder="Seleziona un'azienda"
    />
  );
}
