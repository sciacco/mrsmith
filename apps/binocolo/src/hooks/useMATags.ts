import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ApiError } from '@mrsmith/api-client';
import { useApiClient } from '../api/client';
import type { MATag } from '../api/types';

// Tag aziendali condivisi (issue #198). Modulo centralizzato per il catalogo,
// le mutazioni e le invalidazioni: scheda (overview), elenco (search), board e
// pipeline leggono gli stessi dati e devono muoversi insieme a ogni scrittura.
// Nessuna pagina compone la propria logica tag: tutte le slice future (scheda,
// card, filtro elenco, gestione tag) passano da qui.

// Chiave del catalogo condiviso (GET /binocolo/v1/ma/tags).
export const maTagsCatalogKey = ['ma-tags'] as const;

// Prefissi delle chiavi reali già usate nell'app per le viste alimentate dai
// tag. L'invalidazione usa il prefisso (partial match React Query): le query
// con parametri (`['ma-companies-search', params]`, `['ma-company-overview',
// key]`) cadono tutte sotto il prefisso.
const companiesSearchKey = ['ma-companies-search'] as const;
const companyOverviewKey = (companyKey?: string) =>
  companyKey ? (['ma-company-overview', companyKey] as const) : (['ma-company-overview'] as const);
const boardKey = ['ma-board'] as const;
const pipelineKey = ['ma-pipeline'] as const;

// Politica di freschezza per le query toccate dai tag: il catalogo è condiviso
// tra utenti, quindi le associazioni e i nomi devono essere riletti all'apertura
// della vista e al ritorno sulla finestra, superando localmente i default
// dell'app (staleTime 5 min, refetchOnWindowFocus false — main.tsx). Le slice
// che mostrano tag (overview, elenco, board, pipeline) spargono questo oggetto
// nelle loro useQuery per adottare la stessa politica senza toccare i default.
export const maTagQueryPolicy = {
  staleTime: 0,
  refetchOnMount: 'always',
  refetchOnWindowFocus: true,
} as const;

export function useMATagCatalog() {
  const api = useApiClient();
  return useQuery({
    queryKey: maTagsCatalogKey,
    queryFn: () => api.get<MATag[]>('/binocolo/v1/ma/tags'),
    ...maTagQueryPolicy,
  });
}

// Unica invalidazione per qualsiasi scrittura sui tag. `companyKey` restringe
// l'overview alla scheda interessata; senza (rinomina ed eliminazione globale)
// vengono invalidate tutte le overview in cache, perché il nome del tag cambia
// in ogni vista dove compare.
export async function invalidateMATagViews(
  queryClient: ReturnType<typeof useQueryClient>,
  target?: { companyKey?: string },
) {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: maTagsCatalogKey }),
    queryClient.invalidateQueries({ queryKey: companyOverviewKey(target?.companyKey) }),
    queryClient.invalidateQueries({ queryKey: companiesSearchKey }),
    queryClient.invalidateQueries({ queryKey: boardKey }),
    queryClient.invalidateQueries({ queryKey: pipelineKey }),
  ]);
}

// Rinomina ed eliminazione globale (pagina Gestione tag). La rinomina
// preserva l'UUID: le selezioni per id restano valide. L'eliminazione rimuove
// il tag dal catalogo e da tutte le aziende (CASCADE); le aziende restano.
export function useMATagCatalogMutations() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const invalidate = () => invalidateMATagViews(queryClient);
  const rename = useMutation({
    mutationFn: ({ tagId, name }: { tagId: string; name: string }) =>
      api.put<MATag>(`/binocolo/v1/ma/tags/${encodeURIComponent(tagId)}`, { name }),
    onSuccess: invalidate,
  });
  const remove = useMutation({
    mutationFn: (tagId: string) => api.delete<void>(`/binocolo/v1/ma/tags/${encodeURIComponent(tagId)}`),
    onSuccess: invalidate,
  });
  return { rename, remove };
}

// Associazioni di una singola azienda (scheda). createAndAssign crea il tag nel
// catalogo condiviso e lo assegna in un'unica operazione; assign e unassign
// sono idempotenti lato backend. Ogni esito invalida catalogo, overview
// dell'azienda, elenco, board e pipeline.
export function useMATagCompanyMutations(companyKey: string) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const invalidate = () => invalidateMATagViews(queryClient, { companyKey });
  const root = `/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/tags`;
  const createAndAssign = useMutation({
    mutationFn: (name: string) => api.post<MATag>(root, { name }),
    onSuccess: invalidate,
  });
  const assign = useMutation({
    mutationFn: (tagId: string) => api.put<void>(`${root}/${encodeURIComponent(tagId)}`),
    onSuccess: invalidate,
  });
  const unassign = useMutation({
    mutationFn: (tagId: string) => api.delete<void>(`${root}/${encodeURIComponent(tagId)}`),
    onSuccess: invalidate,
  });
  return { createAndAssign, assign, unassign };
}

// Messaggi italiani consumer-friendly per gli errori delle chiamate tag,
// mappati sui codici del backend (`{"error": "<codice>"}`):
// - ma_tag_name_conflict (409): nome equivalente già presente;
// - ma_tag_not_found (404): tag eliminato nel frattempo (anche da altro utente);
// - invalid_ma_request (400): nome mancante/non valido (o azienda sconosciuta);
// - invalid_ma_tag_id (400): UUID tag malformato.
export function maTagErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    const body = error.body as { error?: string } | undefined;
    switch (body?.error) {
      case 'ma_tag_name_conflict':
        return 'Esiste già un tag con questo nome.';
      case 'ma_tag_not_found':
        return 'Il tag non esiste più: è stato eliminato da un altro utente.';
      case 'invalid_ma_request':
        return 'Nome del tag mancante o non valido.';
      case 'invalid_ma_tag_id':
        return 'Identificativo del tag non valido.';
    }
    if (error.status === 403) return 'Non hai accesso a Binocolo.';
    if (error.status === 503) return 'Servizio non configurato in questo ambiente.';
  }
  return 'Operazione non riuscita. Riprova.';
}

// Parametro ripetibile `tagId` della ricerca aziende (modalità semplice e
// avanzata): sostituisce ogni `tagId` già presente in `params` con la
// selezione corrente (deduplicata, senza vuoti), così la funzione è
// riapplicabile a ogni cambiamento di selezione. Senza selezione il filtro
// sparisce e la ricerca non è ristretta.
export function withMATagFilter(params: string, tagIds: string[]): string {
  const search = new URLSearchParams(params);
  search.delete('tagId');
  const seen = new Set<string>();
  for (const raw of tagIds) {
    const id = raw.trim();
    if (!id || seen.has(id)) continue;
    seen.add(id);
    search.append('tagId', id);
  }
  return search.toString();
}
