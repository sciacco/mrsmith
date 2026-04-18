import styles from '../pages/shared.module.css';

interface ViewStateProps {
  title: string;
  message: string;
  tone?: 'neutral' | 'error';
}

export function ViewState({ title, message, tone = 'neutral' }: ViewStateProps) {
  return (
    <section className={`${styles.statePanel} ${tone === 'error' ? styles.statePanelError : ''}`}>
      <p className={styles.stateEyebrow}>{tone === 'error' ? 'Errore' : 'Workspace'}</p>
      <h2 className={styles.stateTitle}>{title}</h2>
      <p className={styles.stateMessage}>{message}</p>
    </section>
  );
}
