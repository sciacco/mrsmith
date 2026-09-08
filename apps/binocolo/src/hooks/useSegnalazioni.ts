import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import type { Segnalazione, SegnalazioneState, SegnalazioneWrite } from '../api/types';

export const segnalazioniKey = ['segnalazioni'] as const;
const root = '/binocolo/v1/segnalazioni';

export function useSegnalazioni() {
  const api = useApiClient();
  return useQuery({
    queryKey: segnalazioniKey,
    queryFn: () => api.get<Segnalazione[]>(root),
  });
}

export function useSegnalazione(id: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: [...segnalazioniKey, id] as const,
    queryFn: () => api.get<Segnalazione>(`${root}/${encodeURIComponent(id ?? '')}`),
    enabled: Boolean(id),
  });
}

/** Mutazioni sulle segnalazioni. Nessuno stato ottimistico: ogni esito
 *  (anche l'errore del cambio stato) invalida la lista, così la Kanban torna
 *  allo stato effettivamente persistito. */
export function useSegnalazioneMutations() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: segnalazioniKey });
  const create = useMutation({
    mutationFn: (body: SegnalazioneWrite) => api.post<Segnalazione>(root, body),
    onSuccess: invalidate,
  });
  const updateContent = useMutation({
    mutationFn: ({ id, body }: { id: string; body: SegnalazioneWrite }) => api.put<Segnalazione>(`${root}/${encodeURIComponent(id)}`, body),
    onSuccess: invalidate,
    // Un 409 «segnalazione chiusa» significa che lo stato è cambiato da un
    // altro utente: rileggendo, il dettaglio passa in sola lettura.
    onError: invalidate,
  });
  const setState = useMutation({
    mutationFn: ({ id, state }: { id: string; state: SegnalazioneState }) => api.post<Segnalazione>(`${root}/${encodeURIComponent(id)}/state`, { state }),
    onSuccess: invalidate,
    onError: invalidate,
  });
  return { create, updateContent, setState };
}
