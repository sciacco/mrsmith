import type { ProviderSummary } from '../api/types';

export function ProviderSelect({
  providers,
  value,
  disabled,
  onChange,
}: {
  providers: ProviderSummary[];
  value: number | '';
  disabled?: boolean;
  onChange: (value: number | '') => void;
}) {
  return (
    <select value={value} disabled={disabled} onChange={(event) => onChange(event.target.value ? Number(event.target.value) : '')}>
      <option value="">Seleziona fornitore</option>
      {providers.map((provider) => (
        <option key={provider.id} value={provider.id}>
          {provider.company_name ?? `Fornitore ${provider.id}`}
        </option>
      ))}
    </select>
  );
}
