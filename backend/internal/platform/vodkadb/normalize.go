package vodkadb

import (
	"context"
	"database/sql"
)

// NormalizePDFTextFields fills NULL text columns in orders and orders_rows
// with safe defaults so gw-int can scan them into non-null Go strings.
func NormalizePDFTextFields(ctx context.Context, db *sql.DB, orderID int64) error {
	if _, err := db.ExecContext(ctx, ordersNormalizeSQL, orderID); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, ordersRowsNormalizeSQL, orderID)
	return err
}

const ordersNormalizeSQL = `
UPDATE orders
SET cdlan_commerciale          = COALESCE(cdlan_commerciale, ''),
    cdlan_cod_termini_pag      = COALESCE(cdlan_cod_termini_pag, ''),
    cdlan_note                 = COALESCE(cdlan_note, ''),
    cdlan_dur_rin              = COALESCE(cdlan_dur_rin, ''),
    cdlan_tacito_rin           = COALESCE(cdlan_tacito_rin, ''),
    cdlan_sost_ord             = COALESCE(cdlan_sost_ord, ''),
    cdlan_tempi_ril            = COALESCE(cdlan_tempi_ril, ''),
    cdlan_durata_servizio      = COALESCE(cdlan_durata_servizio, ''),
    cdlan_rif_ordcli           = COALESCE(cdlan_rif_ordcli, ''),
    cdlan_rif_tech_nom         = COALESCE(cdlan_rif_tech_nom, ''),
    cdlan_rif_tech_tel         = COALESCE(cdlan_rif_tech_tel, ''),
    cdlan_rif_tech_email       = COALESCE(cdlan_rif_tech_email, ''),
    cdlan_rif_altro_tech_nom   = COALESCE(cdlan_rif_altro_tech_nom, ''),
    cdlan_rif_altro_tech_tel   = COALESCE(cdlan_rif_altro_tech_tel, ''),
    cdlan_rif_altro_tech_email = COALESCE(cdlan_rif_altro_tech_email, ''),
    cdlan_rif_adm_nom          = COALESCE(cdlan_rif_adm_nom, ''),
    cdlan_rif_adm_tech_tel     = COALESCE(cdlan_rif_adm_tech_tel, ''),
    cdlan_rif_adm_tech_email   = COALESCE(cdlan_rif_adm_tech_email, ''),
    cdlan_valuta               = COALESCE(cdlan_valuta, 'EURO'),
    written_by                 = COALESCE(written_by, ''),
    profile_iva                = COALESCE(profile_iva, ''),
    profile_cf                 = COALESCE(profile_cf, ''),
    profile_address            = COALESCE(profile_address, ''),
    profile_city               = COALESCE(profile_city, ''),
    profile_cap                = COALESCE(profile_cap, ''),
    profile_pv                 = COALESCE(profile_pv, ''),
    profile_sdi                = COALESCE(profile_sdi, ''),
    profile_lang               = COALESCE(profile_lang, 'it'),
    service_type               = COALESCE(service_type, ''),
    data_decorrenza            = COALESCE(data_decorrenza, ''),
    cdlan_tacito_rin_in_pdf    = COALESCE(cdlan_tacito_rin_in_pdf, '1'),
    is_colo                    = COALESCE(is_colo, '0'),
    origin_cod_termini_pag     = COALESCE(origin_cod_termini_pag, '')
WHERE id = ?
  AND (
    cdlan_commerciale IS NULL
    OR cdlan_cod_termini_pag IS NULL
    OR cdlan_note IS NULL
    OR cdlan_dur_rin IS NULL
    OR cdlan_tacito_rin IS NULL
    OR cdlan_sost_ord IS NULL
    OR cdlan_tempi_ril IS NULL
    OR cdlan_durata_servizio IS NULL
    OR cdlan_rif_ordcli IS NULL
    OR cdlan_rif_tech_nom IS NULL
    OR cdlan_rif_tech_tel IS NULL
    OR cdlan_rif_tech_email IS NULL
    OR cdlan_rif_altro_tech_nom IS NULL
    OR cdlan_rif_altro_tech_tel IS NULL
    OR cdlan_rif_altro_tech_email IS NULL
    OR cdlan_rif_adm_nom IS NULL
    OR cdlan_rif_adm_tech_tel IS NULL
    OR cdlan_rif_adm_tech_email IS NULL
    OR cdlan_valuta IS NULL
    OR written_by IS NULL
    OR profile_iva IS NULL
    OR profile_cf IS NULL
    OR profile_address IS NULL
    OR profile_city IS NULL
    OR profile_cap IS NULL
    OR profile_pv IS NULL
    OR profile_sdi IS NULL
    OR profile_lang IS NULL
    OR service_type IS NULL
    OR data_decorrenza IS NULL
    OR cdlan_tacito_rin_in_pdf IS NULL
    OR is_colo IS NULL
    OR origin_cod_termini_pag IS NULL
  )`

const ordersRowsNormalizeSQL = `
UPDATE orders_rows
SET cdlan_codart              = COALESCE(cdlan_codart, ''),
    cdlan_descart             = COALESCE(cdlan_descart, ''),
    cdlan_qta                 = COALESCE(cdlan_qta, '1'),
    cdlan_serialnumber        = COALESCE(cdlan_serialnumber, ''),
    cdlan_prezzo              = COALESCE(cdlan_prezzo, '0'),
    cdlan_prezzo_attivazione  = COALESCE(cdlan_prezzo_attivazione, '0'),
    cdlan_prezzo_cessazione   = COALESCE(cdlan_prezzo_cessazione, '0'),
    cdlan_ragg_fatturazione   = COALESCE(cdlan_ragg_fatturazione, 'A'),
    cdlan_codice_kit          = COALESCE(cdlan_codice_kit, '')
WHERE orders_id = ?
  AND (
    cdlan_codart IS NULL
    OR cdlan_descart IS NULL
    OR cdlan_qta IS NULL
    OR cdlan_serialnumber IS NULL
    OR cdlan_prezzo IS NULL
    OR cdlan_prezzo_attivazione IS NULL
    OR cdlan_prezzo_cessazione IS NULL
    OR cdlan_ragg_fatturazione IS NULL
    OR cdlan_codice_kit IS NULL
  )`
