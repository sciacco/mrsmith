import styles from './PreventiviPage.module.css';

export function ArchivioPage() {
  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Vendite</p>
          <h1>Archivio</h1>
        </div>
      </header>

      <section className={styles.emptyPanel} aria-label="Archivio">
        <div className={styles.emptyIcon} aria-hidden="true">
          A
        </div>
        <div className={styles.emptyText}>
          <h2>Nessun documento</h2>
          <p>Archivio storico dei documenti Danea.</p>
        </div>
      </section>
    </main>
  );
}
