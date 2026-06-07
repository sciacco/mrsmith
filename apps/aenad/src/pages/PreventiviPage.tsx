import styles from './PreventiviPage.module.css';

export function PreventiviPage() {
  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Vendite</p>
          <h1>Preventivi</h1>
        </div>
      </header>

      <section className={styles.emptyPanel} aria-label="Preventivi">
        <div className={styles.emptyIcon} aria-hidden="true">
          €
        </div>
        <div className={styles.emptyText}>
          <h2>Nessun preventivo da mostrare</h2>
          <p>I preventivi saranno disponibili qui.</p>
        </div>
      </section>
    </main>
  );
}
