import styles from '../styles/shared.module.css';

interface WorkspaceStubProps {
  eyebrow: string;
  title: string;
  message: string;
}

export function WorkspaceStub({ eyebrow, title, message }: WorkspaceStubProps) {
  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <p className={styles.eyebrow}>{eyebrow}</p>
          <h1 className={styles.title}>{title}</h1>
          <p className={styles.subtitle}>{message}</p>
        </div>
      </div>
      <section className={styles.emptyCard}>
        <p className={styles.stateEyebrow}>Area DCIM</p>
        <h2 className={styles.stateTitle}>{title}</h2>
        <p className={styles.stateMessage}>Questa sezione non e ancora disponibile nel portale.</p>
      </section>
    </section>
  );
}
