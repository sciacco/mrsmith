import type { TeamRow } from '../api/types.js';

export type TeamLabelMap = Record<string, string>;

export function buildTeamLabelMap(teams: TeamRow[] | undefined): TeamLabelMap {
  if (!teams) return {};
  const map: TeamLabelMap = {};
  for (const team of teams) {
    if (team.code) map[team.code] = team.name || team.code;
  }
  return map;
}

function humanizeCode(code: string): string {
  return code
    .toLowerCase()
    .split(/[_\s]+/)
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

export function formatTeamLabel(code: string | undefined, map: TeamLabelMap): string {
  if (!code) return '';
  return map[code] ?? humanizeCode(code);
}
