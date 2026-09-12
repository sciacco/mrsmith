// Rimando da /catalogo (rotta storica, prima della scomposizione nelle
// quattro sottopagine) a /catalogo/corsi, conservando la query string: i
// collegamenti esistenti con ?id=<corso> continuano ad aprire il dettaglio corso.

import { Navigate, useLocation } from 'react-router-dom';

export function CatalogRedirect() {
  const { search } = useLocation();
  return <Navigate replace to={{ pathname: '/catalogo/corsi', search }} />;
}
