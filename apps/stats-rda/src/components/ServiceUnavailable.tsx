export function ServiceUnavailable({ service }: { service: string }) {
  return (
    <div className="surface stateCard">
      <div>
        <p className="stateTitle">Servizio non disponibile</p>
        <p className="muted">La connessione a {service} non è disponibile. Riprova più tardi.</p>
      </div>
    </div>
  );
}
