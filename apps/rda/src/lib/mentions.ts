export function trailingMentionToken(value: string): string | null {
  const match = value.match(/(^|\s)@([^\s@]*)$/);
  return match?.[2] ?? null;
}

export function replaceTrailingMention(value: string, email: string): string {
  return value.replace(/(^|\s)@([^\s@]*)$/, `$1@${email} `);
}
