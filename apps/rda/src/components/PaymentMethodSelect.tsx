import type { PaymentMethod } from '../api/types';

export function PaymentMethodSelect({
  methods,
  value,
  disabled,
  onChange,
}: {
  methods: PaymentMethod[];
  value: string;
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <select value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)}>
      <option value="">Seleziona pagamento</option>
      {methods.map((method) => (
        <option key={method.code} value={method.code}>
          {method.description || method.code}
        </option>
      ))}
    </select>
  );
}
