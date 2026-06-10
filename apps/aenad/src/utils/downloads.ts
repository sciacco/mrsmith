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

// Replica il nome file delle stampe legacy (e del backend):
// "Offerta 1038 del 03-06-2026 RIMOND S R L.pdf".
export function offertaFilename(doc: {
  NumDoc: string | null;
  DataDoc: string | null;
  Anagr_Nome: string | null;
}): string {
  const parts = ['Offerta'];
  if (doc.NumDoc) parts.push(doc.NumDoc);
  const match = doc.DataDoc?.match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (match) parts.push('del', `${match[3]}-${match[2]}-${match[1]}`);
  const cliente = (doc.Anagr_Nome ?? '')
    .replace(/[./\\:*?"<>|]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
  if (cliente) parts.push(cliente);
  return `${parts.join(' ')}.pdf`;
}
