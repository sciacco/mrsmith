// Isolated visual preview. Synthetic data only; no API calls or mutations.
import { Fragment, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Button, Drawer, Icon, SearchInput, SingleSelect, StatusBadge, VisuallyHidden } from '@mrsmith/ui';
import '../../styles/global.css';
import styles from './planning.module.css';

type Lens = 'Operativa' | 'Promemoria' | 'Sospese' | 'Storico';
type Course = {
  id: string; title: string; area: string; tag: string; people: string; priority?: number;
  delivery: string; next: string; economy: string; coverage: string;
  reminder?: { text: string; owner: string; date?: string; day?: string; timing?: 'Scaduto' | 'Oggi' | 'Prossimo' };
  suspended?: boolean; operative?: boolean; history?: boolean; event: string; period: string; facts: string[];
};
const courses: Course[] = [
  { id: 'linux', title: 'Linux · amministrazione sistemi', area: 'Linux & Open source', tag: 'Percorso tecnici', people: '3 persone · Operations', priority: 1, delivery: '1 in corso · 1 pianificata', next: '1 richiesta da decidere', economy: 'RDA approvata', coverage: '2 iscrizioni coperte', reminder: { text: 'Confermare accesso al laboratorio', owner: 'Edizione autunnale', date: '12 ottobre', day: '12', timing: 'Scaduto' }, operative: true, event: 'Edizione autunnale', period: '14–28 ottobre 2026', facts: ['2 iscrizioni collegate all’evento', 'RDA DEMO-101 · approvata', 'Una richiesta resta in attesa della decisione People.'] },
  { id: 'docs', title: 'Comunicazione efficace', area: 'Comunicazione', tag: 'Soft skill', people: '5 persone · più team', delivery: 'Erogazione conclusa', next: 'Documentazione da verificare', economy: 'RDA approvata', coverage: '5 iscrizioni coperte', reminder: { text: 'Verificare gli attestati ricevuti', owner: 'Edizione settembre', date: '15 ottobre', day: '15', timing: 'Oggi' }, operative: true, event: 'Edizione settembre', period: '21–22 settembre 2026', facts: ['5 persone hanno concluso la frequenza.', 'Resta il controllo degli attestati ricevuti.', 'RDA DEMO-105 · approvata'] },
  { id: 'cloud', title: 'Cloud · servizi e casi d’uso', area: 'Cloud', tag: 'Formazione interna', people: '4 persone · Sales', priority: 2, delivery: 'Date da definire', next: 'Formatore già designato', economy: 'Nessuna RDA collegata', coverage: 'Formazione interna', reminder: { text: 'Raccogliere le disponibilità del team', owner: 'Corso', date: '20 ottobre', day: '20', timing: 'Prossimo' }, operative: true, event: 'Formazione interna Sales', period: 'Calendario da definire', facts: ['4 partecipanti previsti', 'Formatore designato: Andrea Ricci (demo)', 'Nessuna sessione ancora fissata.'] },
  { id: 'ccna', title: 'CCNA', area: 'Reti & TLC', tag: 'Percorso tecnici', people: '3 persone · Assurance', delivery: 'Evento pianificato', next: 'Prima sessione il 22 ottobre', economy: 'Nessuna RDA collegata', coverage: '', operative: true, event: 'Edizione ottobre', period: '22–30 ottobre 2026', facts: ['3 iscritti · evento importato da Factorial', 'Nessuna richiesta locale collegata.', 'Prima sessione: 22 ottobre 2026'] },
  { id: 'vmware', title: 'VMware · virtualizzazione', area: 'Virtualizzazione', tag: 'Infrastruttura', people: '2 persone · Operations', delivery: 'Evento pianificato', next: 'L’erogazione resta operativa', economy: 'RDA approvata', coverage: '2 iscrizioni coperte', suspended: true, operative: true, event: 'Edizione autunnale', period: '26 ottobre 2026', facts: ['Istruttoria sospesa: in attesa delle nuove condizioni commerciali.', 'L’evento con 2 iscritti non è stato modificato.', 'RDA DEMO-106 · approvata'] },
  { id: 'cka', title: 'CKA · preparazione ed esame', area: 'Kubernetes', tag: 'Percorso tecnici', people: '1 persona · Operations', delivery: 'Preparazione conclusa', next: 'Esame il 29 ottobre', economy: '2 RDA approvate', coverage: 'Preparazione ed esame', operative: true, event: 'Esame CKA', period: '29 ottobre 2026', facts: ['Preparazione conclusa · RDA DEMO-102', 'Esame pianificato · RDA DEMO-103', 'Nessun conseguimento ancora registrato.'] },
  { id: 'excel', title: 'Excel · analisi dei dati', area: 'Microsoft 365 & Postazioni', tag: 'Strumenti di lavoro', people: '2 persone · Accounting', delivery: 'Erogazione conclusa', next: 'Resta una pendenza economica', economy: 'RDA in approvazione', coverage: '2 iscrizioni collegate', operative: true, event: 'Edizione settembre', period: '24 settembre 2026', facts: ['Frequenza conclusa per entrambe le persone.', 'RDA DEMO-104 · in approvazione', 'La pendenza economica mantiene visibile il corso.'] },
  { id: 'paused', title: 'AI · strumenti per il marketing', area: 'Intelligenza artificiale', tag: 'Soft skill', people: '1 richiesta · Marketing', priority: 3, delivery: 'Istruttoria sospesa', next: 'Nessun evento organizzato', economy: 'Nessuna RDA collegata', coverage: '', suspended: true, reminder: { text: 'Riprendere dopo la revisione delle priorità', owner: 'Corso' }, event: 'Nessun evento', period: 'Tema sospeso', facts: ['Una richiesta in attesa.', 'La riattivazione dell’istruttoria è un gesto esplicito di People.'] },
  { id: 'past', title: 'Sicurezza · aggiornamento', area: 'Sicurezza', tag: 'Compliance', people: '6 persone · più team', delivery: 'Erogazione conclusa', next: 'Nessuna attività residua', economy: 'RDA approvata', coverage: '6 iscrizioni coperte', history: true, event: 'Aggiornamento settembre', period: '10 settembre 2026', facts: ['Tutte le iscrizioni concluse.', 'Nessun promemoria o pendenza economica.'] },
];
const people = [
  { name: 'Giulia Rossi', initials: 'GR', level: '1 → 3', priority: '1', decision: 'Accolta', delivery: 'In corso', variant: 'accent' as const },
  { name: 'Marco Bianchi', initials: 'MB', level: '2 → 3', priority: '2', decision: 'Accolta', delivery: 'Pianificata', variant: 'neutral' as const },
  { name: 'Elena Conti', initials: 'EC', level: '1 → 2', priority: '2', decision: 'Parere TL favorevole', delivery: 'Da decidere', variant: 'warning' as const },
];
const lensCopy: Record<Lens, string> = {
  Operativa: 'Formazione da organizzare, seguire e portare a termine.',
  Promemoria: 'I controlli di oggi e quelli rimasti in sospeso.',
  Sospese: 'Istruttorie in pausa. Gli eventi già pianificati restano invariati.',
  Storico: 'Formazione conclusa, senza attività ancora da seguire.',
};
function Preview() {
  const [lens, setLens] = useState<Lens>('Operativa');
  const [query, setQuery] = useState('');
  const [tag, setTag] = useState('');
  const [open, setOpen] = useState<string | null>('linux');
  const [detail, setDetail] = useState<{ title: string; subtitle: string; facts: string[] } | null>(null);
  const isDue = (c: Course) => c.operative && (c.reminder?.timing === 'Scaduto' || c.reminder?.timing === 'Oggi');
  const filtered = courses.filter(c => (lens === 'Operativa' ? c.operative : lens === 'Promemoria' ? isDue(c) : lens === 'Sospese' ? c.suspended : c.history) && (!tag || c.tag === tag) && `${c.title} ${c.area} ${c.people} ${c.reminder?.text ?? ''}`.toLocaleLowerCase('it').includes(query.toLocaleLowerCase('it')));
  const showEvent = (c: Course) => setDetail({ title: c.event, subtitle: `${c.title} · ${c.period}`, facts: c.facts });
  return <>
    <header className={styles.shell}>
      <div className={styles.brand}><span className={styles.brandIcon}><Icon name="git-branch" size={20}/></span><strong>Formazione</strong><span className={styles.divider}/><span className={styles.workspace}>Area People</span></div>
      <span className={styles.previewLabel}><span/>Anteprima · dati dimostrativi</span>
    </header>
    <main className={styles.page}>
      <header className={styles.heading}>
        <div><div className={styles.breadcrumb}>Lavoro<span>/</span>Pianificazione</div><h1>Pianificazione<span className={styles.titleDot}>.</span></h1><p>{lensCopy[lens]}</p></div>
        <div className={styles.headingAside}><span className={styles.referenceDate}><Icon name="calendar" size={15}/>15 ottobre 2026 <small>· demo</small></span><Button variant="secondary" leftIcon={<Icon name="clock" size={15}/>} onClick={() => { setLens('Promemoria'); setQuery(''); setTag(''); }}>2 promemoria da gestire<Icon name="arrow-right" size={15}/></Button></div>
      </header>
      <section className={styles.panel} aria-label="Pianificazione dei corsi">
        <div className={styles.viewBar}>
          <div className={styles.lenses} aria-label="Vista della pianificazione">{(['Operativa', 'Promemoria', 'Sospese', 'Storico'] as Lens[]).map(l => <Button key={l} className={`${styles.lens} ${lens === l ? styles.lensActive : ''}`} variant="ghost" aria-pressed={lens === l} onClick={() => setLens(l)}>{l}{l === 'Promemoria' && <span className={styles.dueCount}>2</span>}</Button>)}</div>
          <span className={styles.grouping}><Icon name="list-ordered" size={14}/>Per corso</span>
        </div>
        <div className={styles.toolbar}><SearchInput value={query} onChange={setQuery} placeholder="Cerca corso, area o team…"/><div className={styles.tagSelect}><SingleSelect ariaLabel="Filtra per tag" selected={tag || null} onChange={value => setTag(value ?? '')} placeholder="Tutti i tag" options={[...new Set(courses.map(c => c.tag))].map(t => ({ value: t, label: t }))}/></div></div>
        {tag && <div className={styles.context}><span>Filtro attivo</span><Button variant="ghost" size="sm" onClick={() => setTag('')} rightIcon={<Icon name="x" size={13}/>}>{tag}</Button></div>}
        <table className={styles.table}>
          <VisuallyHidden as="caption">Corsi raggruppati con richieste, erogazione, acquisto e promemoria. Apri un corso per il dettaglio.</VisuallyHidden>
          <colgroup><col className={styles.colCourse}/><col className={styles.colProgress}/><col className={styles.colExpense}/><col className={styles.colReminder}/></colgroup>
          <thead><tr><th scope="col">Corso e persone</th><th scope="col">Avanzamento</th><th scope="col">Acquisto</th><th scope="col">Promemoria</th></tr></thead>
          <tbody>{filtered.map(c => <Fragment key={c.id}>
            <tr className={`${styles.summary} ${open === c.id ? styles.selected : ''}`}>
              <td className={styles.identity}><div className={styles.courseLine}><button className={styles.courseButton} aria-expanded={open === c.id} aria-controls={open === c.id ? `detail-${c.id}` : undefined} onClick={() => setOpen(open === c.id ? null : c.id)}><span className={styles.chevron}><Icon name="chevron-right" size={15}/></span><span>{c.title}</span></button>{c.priority && <span className={styles.priority} title={`Priorità più alta delle richieste: ${c.priority}`}><VisuallyHidden>Priorità richieste </VisuallyHidden>P{c.priority}</span>}</div><div className={styles.courseMeta}><span>{c.people}</span><div className={styles.tags}><button onClick={() => setTag(c.tag)} className={styles.tag} aria-label={`Filtra tag ${c.tag}`}>{c.tag}</button>{c.suspended && <span className={styles.suspended}>Corso sospeso</span>}</div></div></td>
              <td data-label="Avanzamento"><div className={styles.delivery}><span className={`${styles.dot} ${c.id === 'linux' ? styles.dotActive : c.delivery.includes('conclusa') ? styles.dotDone : ''}`}/><span>{c.delivery}</span></div><p className={`${styles.secondary} ${c.id === 'linux' ? styles.actionText : ''}`}>{c.next}</p></td>
              <td data-label="Acquisto"><div className={`${styles.economy} ${c.id === 'excel' ? styles.pending : ''}`}>{c.economy.includes('approvat') ? <Icon name="check" size={14}/> : c.id === 'excel' ? <Icon name="clock" size={14}/> : null}<span>{c.economy}</span></div>{c.coverage && <p className={styles.secondary}>{c.coverage}</p>}</td>
              <td data-label="Promemoria">{c.reminder ? <div className={styles.reminder}><div className={`${styles.dateTile} ${isDue(c) ? styles.dateTileDue : ''}`} aria-hidden="true">{c.reminder.day ? <><span>OTT</span><strong>{c.reminder.day}</strong></> : <Icon name="clock" size={18}/>}</div><div><p className={styles.reminderText}>{c.reminder.text}</p><p className={styles.reminderOwner}>{c.reminder.owner}</p>{c.reminder.date && <span className={isDue(c) ? styles.dueText : styles.futureText}><VisuallyHidden>{c.reminder.date} · </VisuallyHidden>{c.reminder.timing === 'Prossimo' ? c.reminder.date : c.reminder.timing}</span>}</div></div> : <span className={styles.noReminder}>—<VisuallyHidden>Nessun promemoria</VisuallyHidden></span>}</td>
            </tr>
            {open === c.id && <tr id={`detail-${c.id}`} className={styles.detail}><td colSpan={4}>
              <div className={styles.detailBody}>
                {c.id === 'linux' ? <div className={styles.peopleSection}><div className={styles.sectionTitle}><h2>Richieste e persone</h2><span>Livelli · Linux</span></div><div className={styles.personRows}>{people.map(p => <div key={p.initials} className={styles.personRow}><span className={styles.avatar}>{p.initials}</span><div className={styles.personName}><strong>{p.name}</strong><span>{p.decision}</span></div><span className={styles.level} aria-label={`Livello attuale e atteso ${p.level}`}>{p.level}</span><StatusBadge value={p.delivery} variant={p.variant} dot/><button className={styles.inspect} aria-label={`Apri richiesta di ${p.name}`} onClick={() => setDetail({ title: `Richiesta di ${p.name}`, subtitle: 'Linux · amministrazione sistemi', facts: [`Operations · priorità ${p.priority}`, `Livello attuale e atteso: ${p.level}`, p.decision, p.delivery === 'Da decidere' ? 'Decisione People da registrare. Nessuna iscrizione collegata.' : `Iscrizione ${p.delivery.toLowerCase()} · Edizione autunnale`] })}><Icon name="arrow-right" size={16}/></button></div>)}</div></div> : <div className={styles.factsSection}><div className={styles.sectionTitle}><h2>Da seguire</h2><span>{c.area}</span></div><p className={styles.focusFact}>{c.next}</p>{c.facts.map(f => <p className={styles.fact} key={f}>{f}</p>)}</div>}
                <aside className={styles.eventCard}><div className={styles.sectionTitle}><h2>{c.id === 'paused' ? 'Istruttoria' : 'Evento'}</h2>{c.id === 'ccna' && <span>Da Factorial</span>}</div><strong className={styles.eventName}>{c.event}</strong><span className={styles.eventDate}><Icon name="calendar" size={14}/>{c.period}</span><div className={styles.eventBottom}><span>{c.id === 'linux' ? '2 iscritti · RDA approvata' : c.coverage || c.delivery}</span><Button variant="ghost" size="sm" rightIcon={<Icon name="arrow-right" size={14}/>} onClick={() => showEvent(c)}>Dettagli</Button></div></aside>
              </div>
            </td></tr>}
          </Fragment>)}</tbody>
        </table>
        {!filtered.length && <div className={styles.empty}><span><Icon name="search" size={24}/></span><h2>Nessun corso corrisponde ai filtri</h2><p>Prova un altro termine o torna alla vista completa.</p><Button variant="secondary" onClick={() => { setQuery(''); setTag(''); }}>Rimuovi filtri</Button></div>}
        <footer className={styles.tableFooter}><span><Icon name="list-ordered" size={13}/>Le attività restano distinte, anche quando condividono un corso.</span><button onClick={() => setOpen(null)} disabled={!open}>Comprimi dettagli<Icon name="chevron-up" size={13}/></button></footer>
      </section>
      <p className={styles.previewFootnote}>Anteprima interattiva · nomi, date e contenuti fittizi · nessun dato viene salvato.</p>
    </main>
    <Drawer open={detail !== null} onClose={() => setDetail(null)} title={detail?.title} subtitle={detail?.subtitle} size="md"><div className={styles.drawerBody}><span className={styles.drawerLabel}>Dettaglio dimostrativo</span>{detail?.facts.map(f => <p key={f}>{f}</p>)}<div className={styles.drawerNotice}><Icon name="info" size={16}/><span>Questa anteprima permette di consultare il contesto. Non registra decisioni o modifiche.</span></div></div></Drawer>
  </>;
}
createRoot(document.getElementById('root')!).render(<Preview/>);
