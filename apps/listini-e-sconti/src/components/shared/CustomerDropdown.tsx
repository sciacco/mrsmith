import { SingleSelect } from '@mrsmith/ui';

interface CustomerOption {
  value: string;
  label: string;
}

interface CustomerDropdownProps {
  options: CustomerOption[];
  selected: string | null;
  onChange: (value: string | null) => void;
  placeholder?: string;
}

export function CustomerDropdown({ options, selected, onChange, placeholder }: CustomerDropdownProps) {
  return (
    <SingleSelect
      options={options}
      selected={selected}
      onChange={onChange}
      placeholder={placeholder ?? 'Seleziona cliente...'}
      allowClear
    />
  );
}

// Helper to map Mistra customers to dropdown options
export function toCustomerOptions(customers: { id: number; name: string }[] | undefined): CustomerOption[] {
  return (customers ?? []).map((c) => ({
    value: String(c.id),
    label: c.name,
  }));
}

// Helper to map Grappa customers to dropdown options
export function toGrappaOptions(customers: { id: number; intestazione: string }[] | undefined): CustomerOption[] {
  return (customers ?? []).map((c) => ({
    value: String(c.id),
    label: c.intestazione,
  }));
}
