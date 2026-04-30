import { Tooltip } from '@mrsmith/ui';
import { stateFullLabel, stateLabel, stateTone } from '../lib/state-labels';

export function StateBadge({ state }: { state?: string | null }) {
  const tone = stateTone(state);
  const label = stateLabel(state);
  return (
    <Tooltip content={stateFullLabel(state)}>
      <span className={`badge ${tone === 'neutral' ? '' : tone}`}>{label}</span>
    </Tooltip>
  );
}
