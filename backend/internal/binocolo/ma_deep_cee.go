package binocolo

// Strato di lettura CEE (Fase 1, DEEP-DIVE-IMPLEMENTATION-PLAN.md): legge gli array
// {code,value} del payload IT-full e deriva le grandezze che entrano nel prezzo, con
// PROVENIENZA dichiarata. Le semantiche codice→voce vengono dalla legend del vendor
// per intero (ma_deep_cee_labels.go, generato): mai assegnate a mano.
//
// Fatti verificati (DEEP-DIVE-DECISIONS.md §1: due fixture reali + inspect n=10):
//   - IIC*/IPL* sono famiglie di codici alternative per divisione di bilancio (stessa
//     numerica, un solo esercizio per payload); la divisione PL non è mai stata
//     osservata sulla cache reale, ma il parsing la accetta.
//   - IPL231/IPL232/IICC351 sono refusi della legend emessi verbatim dall'API:
//     codici canonici, normalizzati a 231/232/351.
//   - Codice base debiti = quota ENTRO l'esercizio, gemello = OLTRE, 329-343 =
//     totali per voce (fatto data-derived, provato dalle identità contabili — la
//     legend non esplicita "entro").
//   - Definizioni vendor riconciliate a rounding su n=10: EBITDA = A(130) − B(149)
//     + B.10(144); PFN = D.1..D.5 + derivati passivi (219) − cassa C.IV (070).
//   - annualResult: 177 = ante imposte, 178 = IMPOSTE, 179 = utile netto.

import (
	"math"
	"strings"
)

// Provenienza di una grandezza derivata: da dove arriva davvero il numero.
const (
	maCEEProvDetail      = "cee_detail"   // codici con split entro/oltre
	maCEEProvTotal       = "cee_total"    // totali per voce (329-343)
	maCEEProvVendorRatio = "vendor_ratio" // derivata dai ratio vendor (ultima spiaggia)
)

// maCEECodes indicizza i valori CEE del payload per NUMERO voce ("094"),
// normalizzando la famiglia: IIC094 e IPL094 sono la stessa voce.
type maCEECodes map[string]float64

// maCEENormalizeCode riduce un codice alla sua numerica. I tre refusi della legend
// propagati dall'API sono canonici: IICC351 → "351" (IPL231/232 rientrano nel caso
// generale IPL). Codici malformati → stringa vuota (scartati).
func maCEENormalizeCode(code string) string {
	if code == "IICC351" {
		return "351"
	}
	if len(code) != 6 {
		return ""
	}
	prefix, num := code[:3], code[3:]
	if prefix != "IIC" && prefix != "IPL" {
		return ""
	}
	for _, r := range num {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return num
}

// maCEECodesFromRoot appiattisce ogni array {code,value} del payload (debts, credits,
// productionCosts, ...) in un'unica mappa numero→valore.
func maCEECodesFromRoot(root map[string]any) maCEECodes {
	out := maCEECodes{}
	for _, value := range root {
		list, ok := value.([]any)
		if !ok {
			continue
		}
		for _, item := range list {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			code, _ := entry["code"].(string)
			num := maCEENormalizeCode(code)
			if num == "" {
				continue
			}
			if n, ok := vendorNumber(entry["value"]); ok {
				out[num] = n
			}
		}
	}
	return out
}

// voce risolve una voce di debito con la catena dettaglio → totale: la somma
// entro+oltre quando i codici split sono depositati, altrimenti il totale per voce.
func (c maCEECodes) voce(base, beyond, total string) (float64, string, bool) {
	within, okWithin := c[base]
	after, okAfter := c[beyond]
	if okWithin || okAfter {
		return within + after, maCEEProvDetail, true
	}
	if v, ok := c[total]; ok {
		return v, maCEEProvTotal, true
	}
	return 0, "", false
}

// maCEEAmount è una grandezza derivata con la sua provenienza.
type maCEEAmount struct {
	Value      float64
	Provenance string
}

// maCEEReading è la lettura deterministica delle voci CEE di un payload IT-full.
// Campi puntatore = voce assente dal deposito; campi valore = 0 se assente (voci
// additive dove l'assenza equivale a zero).
type maCEEReading struct {
	// Debito finanziario ed equity bridge.
	GrossFinancialDebt  *maCEEAmount // D.1+D.2+D.3+D.4+D.5 + derivati passivi (219)
	FinancialDebtBeyond *float64     // quota oltre esercizio (solo con dettaglio pieno)
	PFN                 *maCEEAmount // lordo − cassa − titoli; fallback ratio vendor
	Cash                *float64     // C.IV (070)
	Securities          float64      // C.III.6 altri titoli (065)
	PassiveDerivatives  float64      // B.3 derivati passivi (219)
	TFR                 *float64     // C trattamento fine rapporto (089)
	TaxFund             *float64     // B.2 fondo imposte (086)
	RiskProvisionsTotal *float64     // B totale fondi rischi (088)
	ShareholderLoans    *maCEEAmount // D.3 finanziamenti soci (184+185 | 331)
	ParticipationsTotal *float64     // B.III.1 totale partecipazioni (023)
	ParticipationsSubs  *float64     // B.III.1.a in imprese controllate (019)
	TotalAssets         *float64     // totale attivo (074)
	NetWorth            *float64     // A totale patrimonio netto (084)

	// Conto economico.
	EBITDACEE           *float64 // A(130) − B(149) + B.10(144)
	EBITDAPrudential    *float64 // EBITDACEE − A.4(127) − contributi(129)
	Capitalizations     float64  // A.4 incrementi per lavori interni (127)
	OperatingGrants     float64  // A.5 di cui contributi in conto esercizio (129)
	OtherRevenuesA5     float64  // A.5 totale altri ricavi (128)
	Revenues            *float64 // A.1 ricavi vendite e prestazioni (124)
	LeaseCosts          float64  // B.8 godimento beni di terzi (133)
	InventoryVariation  float64  // B.11 variazione rimanenze materie (145)
	ProvisionsB12B13    float64  // B.12(146) + B.13(147)
	ParticipationIncome float64  // C.15 proventi da partecipazioni (150)

	// Risultato d'esercizio (semantica verificata: 178 = imposte, non utile).
	PreTaxResult *float64 // (177)
	Taxes        *float64 // (178)
	NetProfit    *float64 // (179) = A.IX
}

// maCEEFinancialVoci: le voci del debito finanziario (D.1 obbligazioni, D.2
// convertibili, D.3 soci, D.4 banche, D.5 altri finanziatori) come triple
// base/oltre/totale.
var maCEEFinancialVoci = [][3]string{
	{"090", "091", "329"},
	{"092", "093", "330"},
	{"184", "185", "331"},
	{"094", "095", "332"},
	{"096", "097", "333"},
}

// maCEEReadingFromRoot legge il root (già de-envelopato) di un payload IT-full.
// Ritorna nil quando il payload non porta alcun codice CEE (depositi senza bilancio:
// 2 su 10 nella cache reale al momento della Fase 0).
func maCEEReadingFromRoot(root map[string]any) *maCEEReading {
	codes := maCEECodesFromRoot(root)
	if len(codes) == 0 {
		return nil
	}
	reading := &maCEEReading{}
	optional := func(num string) *float64 {
		if v, ok := codes[num]; ok {
			return &v
		}
		return nil
	}

	// Debito finanziario lordo: somma delle voci D.1-D.5 (catena dettaglio→totale)
	// più i derivati passivi, che la definizione vendor include (verificato su CDLAN).
	grossFound, allDetail := false, true
	gross, beyond := 0.0, 0.0
	for _, f := range maCEEFinancialVoci {
		v, prov, ok := codes.voce(f[0], f[1], f[2])
		if !ok {
			continue
		}
		grossFound = true
		gross += v
		if prov == maCEEProvDetail {
			beyond += codes[f[1]]
		} else {
			allDetail = false
		}
	}
	reading.PassiveDerivatives = codes["219"]
	reading.Cash = optional("070")
	reading.Securities = codes["065"]
	if grossFound {
		prov := maCEEProvTotal
		if allDetail {
			prov = maCEEProvDetail
			b := beyond
			reading.FinancialDebtBeyond = &b
		}
		reading.GrossFinancialDebt = &maCEEAmount{Value: gross + reading.PassiveDerivatives, Provenance: prov}
		if reading.Cash != nil {
			reading.PFN = &maCEEAmount{
				Value:      reading.GrossFinancialDebt.Value - *reading.Cash - reading.Securities,
				Provenance: prov,
			}
		}
	}
	// Ultima spiaggia: la PFN implicita nei ratio vendor (stessa definizione,
	// riconciliata a rounding su n=10) quando i CEE non bastano.
	if reading.PFN == nil {
		if ratio, ok := maInspectNumber(root, "leverageRatios.pfnEbitda"); ok {
			if ebitda, ok := maInspectNumber(root, "operatingResults.ebitda"); ok {
				reading.PFN = &maCEEAmount{Value: ratio * ebitda, Provenance: maCEEProvVendorRatio}
			}
		}
	}

	if v, prov, ok := codes.voce("184", "185", "331"); ok {
		reading.ShareholderLoans = &maCEEAmount{Value: v, Provenance: prov}
	}
	reading.TFR = optional("089")
	reading.TaxFund = optional("086")
	reading.RiskProvisionsTotal = optional("088")
	reading.ParticipationsTotal = optional("023")
	reading.ParticipationsSubs = optional("019")
	reading.TotalAssets = optional("074")
	reading.NetWorth = optional("084")

	// Conto economico: EBITDA CEE e la sua versione prudenziale (al netto dei
	// componenti "fatti in casa": capitalizzazioni A.4 e contributi in esercizio).
	if a, okA := codes["130"]; okA {
		if b, okB := codes["149"]; okB {
			ebitda := a - b + codes["144"]
			reading.EBITDACEE = &ebitda
			prudential := ebitda - codes["127"] - codes["129"]
			reading.EBITDAPrudential = &prudential
		}
	}
	reading.Capitalizations = codes["127"]
	reading.OperatingGrants = codes["129"]
	reading.OtherRevenuesA5 = codes["128"]
	reading.Revenues = optional("124")
	reading.LeaseCosts = codes["133"]
	reading.InventoryVariation = codes["145"]
	reading.ProvisionsB12B13 = codes["146"] + codes["147"]
	reading.ParticipationIncome = codes["150"]

	reading.PreTaxResult = optional("177")
	reading.Taxes = optional("178")
	reading.NetProfit = optional("179")
	return reading
}

// maCEELabel ritorna l'etichetta ufficiale della legend per un codice o numero voce.
func maCEELabel(code string) string {
	num := code
	if strings.HasPrefix(code, "II") || strings.HasPrefix(code, "IP") {
		num = maCEENormalizeCode(code)
	}
	return maCEELabels[num]
}

func maRound2(v float64) float64 {
	return math.Round(v*100) / 100
}

// buildMADeepReconciliation confronta i numeri pre-calcolati dal vendor con la
// rilettura CEE (sanity check, Fase 1). EBITDA in % relativo; PFN in % relativo con
// pavimento 1000€ al denominatore (su PFN prossime a zero il relativo esplode senza
// significato) e SOLO quando la lettura è davvero CEE — confrontare il fallback
// vendor_ratio con sé stesso darebbe sempre zero; ROE in PUNTI percentuali (uno
// scarto relativo esploderebbe sui ROE piccoli). La soglia di allerta
// (vendor_cee_tolerance_pct, calibrata a 1% dall'inspect Fase 0) è consumata dai
// quality flag di Fase 2.
func buildMADeepReconciliation(root map[string]any, reading *maCEEReading) *MADeepReconciliation {
	if reading == nil {
		return nil
	}
	rec := &MADeepReconciliation{}
	found := false
	vendorEBITDA, okVendorEBITDA := maInspectNumber(root, "operatingResults.ebitda")
	if okVendorEBITDA && vendorEBITDA != 0 && reading.EBITDACEE != nil {
		pct := maRound2((*reading.EBITDACEE - vendorEBITDA) / math.Abs(vendorEBITDA) * 100)
		rec.EBITDAPct = &pct
		found = true
	}
	if reading.PFN != nil && reading.PFN.Provenance != maCEEProvVendorRatio && okVendorEBITDA {
		if ratio, ok := maInspectNumber(root, "leverageRatios.pfnEbitda"); ok {
			vendorPFN := ratio * vendorEBITDA
			denom := math.Max(math.Abs(vendorPFN), 1000)
			pct := maRound2((reading.PFN.Value - vendorPFN) / denom * 100)
			rec.PFNPct = &pct
			rec.PFNProvenance = reading.PFN.Provenance
			found = true
		}
	}
	if vendorROE, ok := maInspectNumber(root, "profitability.roe"); ok &&
		reading.NetProfit != nil && reading.NetWorth != nil && *reading.NetWorth != 0 {
		points := maRound2(*reading.NetProfit / *reading.NetWorth * 100 - vendorROE)
		rec.ROEPointsDiff = &points
		found = true
	}
	if !found {
		return nil
	}
	return rec
}
