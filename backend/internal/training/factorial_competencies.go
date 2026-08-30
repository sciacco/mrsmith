package training

import "strings"

// Catalogo competenze Factorial. L'API pubblica (prefisso 2026-07-01) espone
// sui Training solo i competency_ids numerici e nessuna risorsa di catalogo
// con i nomi (verificata l'intera superficie del client; gli attributi
// competency del job catalog risultano vuoti). La mappa riproduce il
// catalogo completo (15 voci) letto il 2026-08-30 dalla GraphQL interna del
// browser Factorial, dopo una prima ricostruzione per incastro dalle schede
// corso coincisa al 100%. Un id assente da qui genera il warning
// course_competency_unresolved: si risolve aggiungendo la riga col nome
// letto dalla UI.
var factorialCompetencyNames = map[string]string{
	"40199": "Comunicazione efficace",
	"40208": "Team working",
	"40209": "Self Motivation",
	"40210": "Flessibilità",
	"40211": "Proattività",
	"40212": "Adattabilità",
	"40213": "Responsabilità",
	"40214": "Precisione",
	"40215": "Problem Solving",
	"40216": "Decision Making",
	"40217": "Organizzazione",
	"71822": "Gestione delle scorte",
	"71823": "Negoziazione",
	"71824": "Analisi dei dati",
	"71825": "Pianificazione della domanda",
}

// skillAreaCode deriva il codice anagrafica dal nome competenza: minuscole,
// spazi compressi in trattini. Il codice e' la chiave del get-or-create
// dell'import: il nome resta rinominabile in app senza creare doppioni.
func skillAreaCode(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(name)), "-")
}
