export function downloadBlob(blob: Blob, filename: string) {
  const href = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = href;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(href);
}

export function safeFilenamePart(value: string): string {
  return value.replace(/[\\/\s]+/g, '_').replace(/[^a-zA-Z0-9_.-]/g, '');
}
