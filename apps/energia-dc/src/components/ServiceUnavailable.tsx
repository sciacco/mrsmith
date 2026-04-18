import { ViewState } from './ViewState';

export function ServiceUnavailable() {
  return (
    <ViewState
      title="Servizio non disponibile"
      message="La base dati Grappa non e configurata. Questa area non e al momento disponibile."
      tone="error"
    />
  );
}
