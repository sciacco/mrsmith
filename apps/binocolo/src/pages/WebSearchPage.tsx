import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type FormEvent,
  type KeyboardEvent,
  type ReactNode,
} from 'react';
import { ApiError } from '@mrsmith/api-client';
import { useMutation } from '@tanstack/react-query';
import { Button, Icon } from '@mrsmith/ui';
import { useApiClient } from '../api/client';
import type { WebSearchResponse } from '../api/types';
import styles from './WebSearchPage.module.css';

const DEFAULT_COUNT = 25;
const MIN_COUNT = 1;
const MAX_COUNT = 50;

function splitKeywords(raw: string): string[] {
  return raw
    .split(',')
    .map((p) => p.trim())
    .filter(Boolean);
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// highlight wraps any occurrence of the searched keywords in <mark>, case-insensitive.
function highlight(text: string, keywords: string[]): ReactNode[] {
  const terms = keywords.map((k) => k.trim()).filter(Boolean).map(escapeRegExp);
  if (terms.length === 0) return [text];
  const re = new RegExp(`(${terms.join('|')})`, 'gi');
  // String.split with a capture group keeps the delimiters: odd indices are matches.
  return text.split(re).map((part, i) =>
    i % 2 === 1 ? (
      <mark key={i} className={styles.mark}>
        {part}
      </mark>
    ) : (
      part
    ),
  );
}

// Snippet clamps long text to a few lines and reveals a toggle only when the text
// actually overflows that clamp.
function Snippet({ text, keywords }: { text: string; keywords: string[] }) {
  const [expanded, setExpanded] = useState(false);
  const [overflowing, setOverflowing] = useState(false);
  const ref = useRef<HTMLParagraphElement>(null);

  useLayoutEffect(() => {
    if (expanded) return;
    const el = ref.current;
    if (el) setOverflowing(el.scrollHeight > el.clientHeight + 1);
  }, [text, expanded]);

  useEffect(() => {
    function measure() {
      if (expanded) return;
      const el = ref.current;
      if (el) setOverflowing(el.scrollHeight > el.clientHeight + 1);
    }
    window.addEventListener('resize', measure);
    return () => window.removeEventListener('resize', measure);
  }, [expanded]);

  return (
    <div className={styles.snippet}>
      <p ref={ref} className={`${styles.snippetText} ${expanded ? '' : styles.snippetClamped}`}>
        {highlight(text, keywords)}
      </p>
      {overflowing || expanded ? (
        <button type="button" className={styles.snippetToggle} onClick={() => setExpanded((v) => !v)}>
          {expanded ? 'Riduci' : 'Mostra tutto'}
        </button>
      ) : null}
    </div>
  );
}

function errorLabel(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 400) return 'Controlla il dominio e le keyword inserite.';
    if (error.status === 403) return 'Non hai accesso a Binocolo.';
    if (error.status === 502) return 'Brave non ha risposto correttamente.';
    if (error.status === 503) return 'Ricerca web non configurata in questo ambiente.';
    return `Richiesta non riuscita (${error.status}).`;
  }
  return 'Richiesta non riuscita.';
}

export function WebSearchPage() {
  const api = useApiClient();
  const [domain, setDomain] = useState('');
  const [keywords, setKeywords] = useState<string[]>([]);
  const [draft, setDraft] = useState('');
  const [count, setCount] = useState(DEFAULT_COUNT);
  const [localError, setLocalError] = useState<string | null>(null);
  const [searchedKeywords, setSearchedKeywords] = useState<string[]>([]);

  const search = useMutation({
    mutationFn: (body: { domain: string; keywords: string[]; count: number }) =>
      api.post<WebSearchResponse>('/binocolo/v1/web-search', body),
  });

  function addKeywords(raw: string) {
    const parts = splitKeywords(raw);
    if (parts.length === 0) return;
    setKeywords((prev) => {
      const next = [...prev];
      for (const p of parts) {
        if (!next.includes(p)) next.push(p);
      }
      return next;
    });
    setDraft('');
  }

  function handleKeywordKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Enter' || event.key === ',') {
      event.preventDefault();
      addKeywords(draft);
    } else if (event.key === 'Backspace' && draft === '' && keywords.length > 0) {
      setKeywords((prev) => prev.slice(0, -1));
    }
  }

  function removeKeyword(index: number) {
    setKeywords((prev) => prev.filter((_, i) => i !== index));
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedDomain = domain.trim();
    const effective = Array.from(new Set([...keywords, ...splitKeywords(draft)]));
    if (!trimmedDomain) {
      setLocalError('Inserisci un dominio, es. azienda.it.');
      return;
    }
    if (effective.length === 0) {
      setLocalError('Inserisci almeno una keyword.');
      return;
    }
    setLocalError(null);
    setKeywords(effective);
    setSearchedKeywords(effective);
    setDraft('');
    search.mutate({ domain: trimmedDomain, keywords: effective, count });
  }

  const canSubmit = domain.trim() !== '' && (keywords.length > 0 || draft.trim() !== '');
  const data = search.data;

  return (
    <div className={styles.page}>
      <div className={styles.pageHeader}>
        <p className={styles.eyebrow}>Ricerca web</p>
        <h1 className={styles.pageTitle}>Ricerca web</h1>
        <p className={styles.pageSubtitle}>Cerca una o più keyword dentro un sito specifico.</p>
      </div>

      <form className={styles.form} onSubmit={handleSubmit}>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>Dominio</span>
          <input
            type="text"
            className={styles.input}
            value={domain}
            onChange={(e) => setDomain(e.target.value)}
            placeholder="azienda.it"
            autoComplete="off"
            spellCheck={false}
            aria-label="Dominio del sito"
          />
        </label>

        <label className={styles.field}>
          <span className={styles.fieldLabel}>Keyword</span>
          <div className={styles.chipsInput}>
            {keywords.map((k, i) => (
              <span key={`${k}-${i}`} className={styles.chip}>
                {k}
                <button
                  type="button"
                  className={styles.chipRemove}
                  onClick={() => removeKeyword(i)}
                  aria-label={`Rimuovi ${k}`}
                >
                  <Icon name="x" size={12} />
                </button>
              </span>
            ))}
            <input
              type="text"
              className={styles.chipDraft}
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={handleKeywordKeyDown}
              onBlur={() => addKeywords(draft)}
              placeholder={keywords.length === 0 ? 'Invio per aggiungere…' : ''}
              autoComplete="off"
              spellCheck={false}
              aria-label="Aggiungi keyword"
            />
          </div>
        </label>

        <label className={`${styles.field} ${styles.fieldCount}`}>
          <span className={styles.fieldLabel}>Risultati</span>
          <input
            type="number"
            className={styles.input}
            value={count}
            min={MIN_COUNT}
            max={MAX_COUNT}
            onChange={(e) => {
              const n = Number(e.target.value);
              if (!Number.isFinite(n)) return;
              setCount(Math.max(MIN_COUNT, Math.min(MAX_COUNT, Math.trunc(n))));
            }}
            aria-label="Numero di risultati"
          />
        </label>

        <Button type="submit" loading={search.isPending} disabled={!canSubmit} leftIcon={<Icon name="search" />}>
          Cerca
        </Button>
      </form>
      {localError ? <p className={styles.localError}>{localError}</p> : null}

      {search.isError ? (
        <div className={styles.statePanel} role="alert">
          <div className={styles.stateIcon}>
            <Icon name="triangle-alert" size={22} />
          </div>
          <p className={styles.stateTitle}>Ricerca non riuscita</p>
          <p className={styles.stateText}>{errorLabel(search.error)}</p>
        </div>
      ) : null}

      {data ? (
        data.results.length > 0 ? (
          <div className={styles.results}>
            <p className={styles.resultsMeta}>
              {data.results.length} risultati · <span className={styles.mono}>{data.query}</span>
            </p>
            {data.results.map((r, i) => (
              <article key={`${r.url}-${i}`} className={styles.result}>
                <a className={styles.resultTitle} href={r.url} target="_blank" rel="noreferrer">
                  {r.title || r.url}
                </a>
                <div className={styles.resultSource}>
                  <span className={styles.mono}>{r.hostname}</span>
                  {r.age ? <span>· {r.age}</span> : null}
                </div>
                {r.snippets.length > 0 ? (
                  <Snippet text={r.snippets.join(' … ')} keywords={searchedKeywords} />
                ) : null}
              </article>
            ))}
          </div>
        ) : (
          <div className={styles.statePanel}>
            <div className={styles.stateIcon}>
              <Icon name="search" size={22} />
            </div>
            <p className={styles.stateTitle}>Nessun risultato nel sito</p>
            <p className={styles.stateText}>Prova con keyword diverse o un altro dominio.</p>
          </div>
        )
      ) : null}
    </div>
  );
}
