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

// Modi della sincronizzazione Factorial: import_only sospende ogni scrittura
// verso Factorial (propagazione cancellazioni ed export), full le abilita.
const (
	factorialSyncModeImportOnly = "import_only"
	factorialSyncModeFull       = "full"
)

// factorialSyncMode legge da mrsmith.runtime_config la riga
// ('training','factorial_sync_mode'). Riga assente o vuota = import_only:
// nessuna scrittura verso Factorial finche' qualcuno non configura
// esplicitamente full (sicuro per difetto). Valore sconosciuto = errore,
// mai un ripiego silenzioso su un modo che scrive.
func (s *SQLStore) factorialSyncMode(ctx context.Context) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("training database not configured")
	}
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
SELECT value
FROM mrsmith.runtime_config
WHERE namespace = 'training' AND key = 'factorial_sync_mode'`,
	).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return factorialSyncModeImportOnly, nil
	}
	if err != nil {
		return "", fmt.Errorf("read training factorial sync mode config: %w", err)
	}
	var asString string
	if json.Unmarshal(raw, &asString) != nil {
		return "", errors.New("invalid training factorial sync mode config: attesa stringa JSON")
	}
	switch mode := strings.TrimSpace(asString); mode {
	case "", factorialSyncModeImportOnly:
		return factorialSyncModeImportOnly, nil
	case factorialSyncModeFull:
		return factorialSyncModeFull, nil
	default:
		return "", fmt.Errorf("invalid training factorial sync mode config: %q (ammessi %s, %s)", mode, factorialSyncModeImportOnly, factorialSyncModeFull)
	}
}
