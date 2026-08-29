package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// factorialAuthorEmployeeID legge da mrsmith.runtime_config la riga
// ('training','factorial_author_employee_id'): l'employee Factorial autore
// delle scritture del sync formativo. Configurazione a runtime,
// inserita dall'utente e modificabile senza riavvio; riga assente o vuota
// restituisce stringa vuota senza errore, la decisione spetta al chiamante.
// Il valore jsonb e' accettato come stringa JSON o come numero.
func (s *SQLStore) factorialAuthorEmployeeID(ctx context.Context) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("training database not configured")
	}
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
SELECT value
FROM mrsmith.runtime_config
WHERE namespace = 'training' AND key = 'factorial_author_employee_id'`,
	).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read training factorial author config: %w", err)
	}
	var asString string
	if json.Unmarshal(raw, &asString) == nil {
		return strings.TrimSpace(asString), nil
	}
	var asNumber json.Number
	if json.Unmarshal(raw, &asNumber) == nil {
		return asNumber.String(), nil
	}
	return "", errors.New("invalid training factorial author config: attesa stringa o numero JSON")
}
