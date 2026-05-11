export function trailingMentionToken(value: string): string | null {
  const match = value.match(/(^|\s)@([^\s@]*)$/);
  return match?.[2] ?? null;
}

export function replaceTrailingMention(value: string, email: string): string {
  return value.replace(/(^|\s)@([^\s@]*)$/, `$1@${email} `);
}

export function hasMentionToken(value: string, email: string): boolean {
  const normalizedEmail = email.trim();
  if (!normalizedEmail) return false;
  const pattern = new RegExp(`(^|\\s)@${escapeRegExp(normalizedEmail)}(?=$|\\s|[.,;:!?])`, 'i');
  return pattern.test(value);
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
