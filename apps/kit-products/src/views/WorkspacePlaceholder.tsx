import styles from './WorkspacePlaceholder.module.css';

interface WorkspacePlaceholderProps {
  eyebrow: string;
  title: string;
  description: string;
}

const deliverables = [
  'bootstrap auth su /config',
  'routing con basename Vite',
  'shell condivisa con tab principali',
  'toast warning condiviso per i flussi ERP',
];

export function WorkspacePlaceholder({
  eyebrow,
  title,
  description,
}: WorkspacePlaceholderProps) {
  return (
    <section className={styles.page}>
      <div className={styles.hero}>
        <div>
          <p className={styles.eyebrow}>{eyebrow}</p>
          <h1>{title}</h1>
          <p className={styles.description}>{description}</p>
        </div>
        <div className={styles.statusCard}>
          <span className={styles.statusLabel}>Foundation</span>
          <strong>Shell pronta</strong>
          <p>Il modulo e raggiungibile e puo essere riempito fase per fase senza ripartire dal wiring.</p>
        </div>
      </div>

      <div className={styles.grid}>
        {deliverables.map((item) => (
          <article key={item} className={styles.card}>
            <span className={styles.cardMarker} />
            <p>{item}</p>
          </article>
        ))}
      </div>
    </section>
  );
}
