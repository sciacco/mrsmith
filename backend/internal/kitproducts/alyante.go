package kitproducts

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const alyanteSyncTranslationSQL = `
UPDATE MG87_ARTDESC
SET MG87_DESCART = @p1
WHERE MG87_DITTA_CG18 = 1
  AND MG87_OPZIONE_MG5E = '                    '
  AND MG87_LINGUA_MG52 = @p3
  AND MG87_CODART_MG66 = @p2
`

type AlyanteAdapter struct {
	db     *sql.DB
	syncFn func(context.Context, string, string, string) error
	execFn func(context.Context, string, ...any) (sql.Result, error)
}

func NewAlyanteAdapter(db *sql.DB) *AlyanteAdapter {
	return &AlyanteAdapter{db: db}
}

func (a *AlyanteAdapter) SyncTranslation(ctx context.Context, code, lang, short string) error {
	if a != nil && a.syncFn != nil {
		return a.syncFn(ctx, code, lang, short)
	}
	if a == nil || (a.db == nil && a.execFn == nil) {
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
	execFn := a.db.ExecContext
	if a.execFn != nil {
		execFn = a.execFn
	}
	_, err := execFn(ctx, alyanteSyncTranslationSQL, short, paddedCode, erpLang)
	if err != nil {
		return fmt.Errorf("sync alyante translation: %w", err)
	}

	return nil
}
