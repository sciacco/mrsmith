// Data layer della board v2 (react-query). Query della board con polling dossier
// condizionale (5s solo se un'analisi è "working") e mutazioni OPTIMISTIC per lo
// spostamento di stato — la card si muove subito, rollback su errore, invalidation
// mirata al termine (KANBAN-V2-PLAN.md §6.2). Le patch di refetch non fanno saltare
// la board (structural sharing di react-query; niente entrance su refetch, §8.3).

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useToast } from '@mrsmith/ui';
import { useApiClient } from '../../../api/client';
import { maTagQueryPolicy } from '../../../hooks/useMATags';
import { errorLabel } from '../../ricerche/helpers';
import type {
  MACardCloseResponse,
  MACardRemoveResponse,
  MACreateInitiativeCardResponse,
  MAInitiativeBoard,
  MAInitiativeCard,
  MAInitiativeCardView,
  MASessionListResponse,
} from '../../../api/types';

export type DossierState = 'none' | 'working' | 'ready' | 'failed' | 'unknown';

export function dossierState(status?: string): DossierState {
  const s = (status ?? '').toLowerCase();
  if (s === 'ready') return 'ready';
  if (s === 'working' || s === 'queued' || s === 'running') return 'working';
  if (s === 'failed') return 'failed';
  if (!s || s === 'none' || s === 'absent') return 'none';
  return 'unknown';
}

function boardHasWorking(board?: MAInitiativeBoard): boolean {
  return board?.cards.some((c) => dossierState(c.dossierStatus) === 'working') ?? false;
}

export function boardQueryKey(initiativeId: string) {
  return ['ma-board', initiativeId] as const;
}

export interface CloseCardInput {
  companyKey: string;
  state: string; // terminale: won | ko_nostro | ko_target
  esito?: string;
  note?: string;
  registerFacts?: string[];
}

export interface CreateDirectInput {
  vatCode?: string;
  companyKey?: string;
  domain?: string;
  initialState?: string;
}

export function useBoardData(initiativeId: string) {
  const api = useApiClient();
  const qc = useQueryClient();
  const { toast } = useToast();
  const key = boardQueryKey(initiativeId);

  const query = useQuery({
    queryKey: key,
    enabled: Boolean(initiativeId),
    queryFn: () => api.get<MAInitiativeBoard>(`/binocolo/v1/ma/initiatives/${initiativeId}`),
    // Polling condizionale: solo mentre un dossier è in elaborazione.
    refetchInterval: (q) => (boardHasWorking(q.state.data) ? 5000 : false),
    // I tag sulle card sono condivisi tra utenti: riletti a apertura e focus
    // (politica centralizzata, issue #198); i default dell'app restano per il resto.
    ...maTagQueryPolicy,
  });

  const cardsUrl = (companyKey: string) =>
    `/binocolo/v1/ma/initiatives/${initiativeId}/cards/${encodeURIComponent(companyKey)}`;

  function patchCard(companyKey: string, patch: Partial<MAInitiativeCardView>) {
    qc.setQueryData<MAInitiativeBoard>(key, (b) =>
      b ? { ...b, cards: b.cards.map((c) => (c.companyKey === companyKey ? { ...c, ...patch } : c)) } : b,
    );
  }

  // Spostamento di stato OPTIMISTIC (drag, tasti 1..8, azione drawer). Il terzo
  // argomento di onSetState trasporta la data opzionale dello stato attivo:
  // `recontactOn` in `ricontattare`, `visitOn` in `visita`.
  const setState = useMutation({
    mutationFn: (v: { companyKey: string; state: string; recontactOn?: string | null }) => {
      const body: { state: string; recontactOn?: string | null; visitOn?: string | null } = { state: v.state };
      // Tri-stato anche sul filo: `undefined` lascia la proprietà assente
      // (riselezionando `visita` il backend conserva la data corrente),
      // `null` la cancella esplicitamente, la data ISO la imposta.
      if (v.recontactOn !== undefined) {
        if (v.state === 'visita') body.visitOn = v.recontactOn;
        else body.recontactOn = v.recontactOn;
      }
      return api.post<MAInitiativeCard>(`${cardsUrl(v.companyKey)}/state`, body);
    },
    onMutate: async (v) => {
      await qc.cancelQueries({ queryKey: key });
      const prev = qc.getQueryData<MAInitiativeBoard>(key);
      const current = prev?.cards.find((c) => c.companyKey === v.companyKey);
      patchCard(v.companyKey, {
        state: v.state,
        // recontact_on vive solo in `ricontattare`; ogni altra transizione la azzera.
        recontactOn: v.state === 'ricontattare' ? (v.recontactOn ?? undefined) : undefined,
        // visit_on vive solo in `visita`: riselezionando senza data il valore
        // corrente resta, ogni altra transizione lo azzera subito (come il backend).
        visitOn:
          v.state === 'visita'
            ? v.recontactOn === undefined
              ? current?.state === 'visita'
                ? current.visitOn
                : undefined
              : (v.recontactOn ?? undefined)
            : undefined,
      });
      return { prev };
    },
    onSuccess: (card) => {
      // La risposta è l'autorità sulla card: il valore confermato sostituisce
      // subito quello ottimistico, senza attendere il refetch dell'invalidation.
      patchCard(card.companyKey, { state: card.state, recontactOn: card.recontactOn, visitOn: card.visitOn });
    },
    onError: (e, _v, ctx) => {
      if (ctx?.prev) qc.setQueryData(key, ctx.prev);
      toast(errorLabel(e), 'error');
    },
    onSettled: () => qc.invalidateQueries({ queryKey: key }),
  });

  // Transizione terminale (WON conferma secca / KO esito+ponte). Non-optimistic:
  // il modale conferma prima, e il set di card cambia macrofase.
  const closeCard = useMutation({
    mutationFn: (v: CloseCardInput) =>
      api.post<MACardCloseResponse>(`${cardsUrl(v.companyKey)}/close`, {
        state: v.state,
        esito: v.esito || undefined,
        note: v.note || undefined,
        registerFacts: v.registerFacts && v.registerFacts.length > 0 ? v.registerFacts : undefined,
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
  });

  const removeCard = useMutation({
    mutationFn: (v: { companyKey: string; correctRating: boolean; reason?: string }) =>
      api.post<MACardRemoveResponse>(`${cardsUrl(v.companyKey)}/remove`, {
        correctRating: v.correctRating,
        reason: v.reason || undefined,
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
  });

  const reopenCard = useMutation({
    mutationFn: (companyKey: string) => api.post(`${cardsUrl(companyKey)}/reopen`, {}),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
    onError: (e) => toast(errorLabel(e), 'error'),
  });

  const startDeepDive = useMutation({
    mutationFn: (companyKey: string) =>
      api.post(`/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/deep-dive`, {}),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
    onError: (e) => toast(errorLabel(e), 'error'),
  });

  const createDirect = useMutation({
    mutationFn: (v: CreateDirectInput) =>
      api.post<MACreateInitiativeCardResponse>(`/binocolo/v1/ma/initiatives/${initiativeId}/cards`, v),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
  });

  const attachSession = useMutation({
    mutationFn: (sessionId: string) =>
      api.post(`/binocolo/v1/ma/sessions/${sessionId}/initiative`, { initiativeId }),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
  });

  async function listAttachableSessions(): Promise<MASessionListResponse['items']> {
    const data = await api.get<MASessionListResponse>('/binocolo/v1/ma/sessions');
    const anchored = new Set((query.data?.sessions ?? []).map((s) => s.id));
    return data.items.filter((s) => !s.initiativeId && !anchored.has(s.id));
  }

  return {
    board: query.data ?? null,
    loading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
    setState,
    closeCard,
    removeCard,
    reopenCard,
    startDeepDive,
    createDirect,
    attachSession,
    listAttachableSessions,
  };
}
