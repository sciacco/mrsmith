import { Navigate } from 'react-router-dom';

export function HomeRedirect() {
  return <Navigate to="/richieste" replace />;
}
