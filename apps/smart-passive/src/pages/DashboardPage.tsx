import styles from './DashboardPage.module.css';

export function DashboardPage() {
  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <p className={styles.eyebrow}>Ciclo passivo</p>
        <h1>Smart Passive</h1>
        <p className={styles.description}>
          Controllo del ciclo passivo: abbinamento fatture agli ordini di acquisto,
          gestione delle anomalie e monitoraggio dei consumi di budget.
        </p>
      </div>
      <div className={styles.statusCard}>
        <span className={styles.statusLabel}>In arrivo</span>
        <strong>Modulo in allestimento</strong>
        <p>
          La shell &egrave; pronta. Le pagine di abbinamento, anomalie e consumi
          verranno aggiunte nelle prossime iterazioni.
        </p>
      </div>
    </section>
  );
}
