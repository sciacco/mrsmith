package kitproducts

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type AlyanteAdapter struct {
	db     *sql.DB
	syncFn func(context.Context, string, string, string) error
}

func NewAlyanteAdapter(db *sql.DB) *AlyanteAdapter {
	return &AlyanteAdapter{db: db}
}

func (a *AlyanteAdapter) SyncTranslation(ctx context.Context, code, lang, short string) error {
	if a != nil && a.syncFn != nil {
		return a.syncFn(ctx, code, lang, short)
	}
	if a == nil || a.db == nil {
		return nil
	}

	erpLang := map[string]string{
		"it": "ITA",
		"en": "ING",
	}[strings.ToLower(strings.TrimSpace(lang))]
	if erpLang == "" {
		return fmt.Errorf("unsupported ERP language %q", lang)
	}

	paddedCode := fmt.Sprintf("%-25s", code)
	_, err := a.db.ExecContext(ctx, `
UPDATE MG87_ARTDESC
SET MG87_DESCRIZIONE = @p1
WHERE MG87_CODART = @p2
  AND MG87_LINGUA = @p3
  AND MG87_OPZIONE = '                    '
  AND MG87_DITTA = 1
`, short, paddedCode, erpLang)
	if err != nil {
		return fmt.Errorf("sync alyante translation: %w", err)
	}

	return nil
}
