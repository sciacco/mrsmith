import { SingleSelect } from '@mrsmith/ui';
import type { ProviderSummary } from '../api/types';

export function ProviderCombobox({
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
    <SingleSelect<number>
      options={providers.map((provider) => ({
        value: provider.id,
        label: provider.company_name ?? `Fornitore ${provider.id}`,
      }))}
      selected={value === '' ? null : value}
      disabled={disabled}
      placeholder="Seleziona fornitore"
      onChange={(next) => onChange(next ?? '')}
    />
  );
}
