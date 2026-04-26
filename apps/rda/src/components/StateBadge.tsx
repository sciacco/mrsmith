import { stateLabel, stateTone } from '../lib/state-labels';

export function StateBadge({ state }: { state?: string | null }) {
  const tone = stateTone(state);
  return <span className={`badge ${tone === 'neutral' ? '' : tone}`}>{stateLabel(state)}</span>;
}
