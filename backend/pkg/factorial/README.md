# Utilizzo del client Factorial

Questa directory contiene un client Go autonomo per le API pubbliche di
Factorial. Usa esclusivamente la standard library e può essere copiata
direttamente all'interno di un altro modulo Go.

La versione inclusa usa il prefisso API Factorial:

```text
/api/2026-07-01
```

## Installazione tramite copia

Copia l'intera directory `factorial` in una directory a scelta del modulo Go
che deve usarla. La posizione non è imposta dalla libreria.

Sono valide, per esempio:

```text
progetto/factorial/           # package alla radice del modulo
progetto/pkg/factorial/       # convenzione per package riutilizzabili
progetto/internal/factorial/  # import consentito soltanto dall'albero padre
```

`pkg` è soltanto una convenzione organizzativa. `internal`, invece, ha un
significato per il compilatore Go e limita da dove il package può essere
importato.

La directory scelta deve contenere tutti i file della libreria:

```text
factorial/
├── README.md
├── client.go
├── date.go
├── doc.go
├── errors.go
├── multipart.go
├── pagination.go
├── namespaces.gen.go
├── types.gen.go
└── webhooks.gen.go
```

I file `*_test.go` possono essere copiati per conservare la suite di test, ma
non sono necessari per compilare e usare il client.

I tre file generati `namespaces.gen.go`, `types.gen.go` e `webhooks.gen.go`
sono parte integrante della libreria e non possono essere omessi.

Non sono necessarie direttive `require` o `replace` quando la directory viene
copiata nello stesso modulo.

L'import path si ottiene concatenando:

```text
<valore della direttiva module>/<percorso della directory nel modulo>
```

Se, per esempio, il `go.mod` contiene:

```go
module github.com/acme/backend

go 1.24
```

gli import corrispondenti alle tre possibili collocazioni sono:

```go
import "github.com/acme/backend/factorial"          // factorial/
import "github.com/acme/backend/pkg/factorial"      // pkg/factorial/
import "github.com/acme/backend/internal/factorial" // internal/factorial/
```

Negli esempi successivi viene usato, a puro titolo illustrativo,
`github.com/acme/backend/pkg/factorial`. Sostituirlo con l'import path
calcolato per il proprio progetto.

## Creazione del client e autenticazione

Il client può leggere automaticamente la configurazione dall'ambiente:

```go
client := factorial.New()
```

Variabili riconosciute:

- `FACTORIAL_API_KEY`: inviata nell'header `x-api-key`;
- `FACTORIAL_TOKEN`: inviato come `Authorization: Bearer <token>`;
- `FACTORIAL_BASE_URL`: sostituisce `https://api.factorialhr.com`.

La configurazione può essere fornita esplicitamente:

```go
client := factorial.New(
	factorial.WithAPIKey(os.Getenv("FACTORIAL_API_KEY")),
)
```

Sono disponibili anche:

```go
factorial.WithToken(token)
factorial.WithBaseURL(baseURL)
factorial.WithHTTPClient(httpClient)
factorial.WithUserAgent(userAgent)
```

Le opzioni esplicite hanno precedenza sulle variabili d'ambiente. È possibile
fornire sia una API key sia un token; in quel caso vengono inviati entrambi.

## Prima richiesta

L'accesso alle operazioni rispecchia la struttura delle API:

```text
client.<Namespace>.<Risorsa>.<Operazione>
```

Esempio: lettura di una pagina di dipendenti attivi.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/acme/backend/pkg/factorial"
)

func main() {
	client := factorial.New()

	page, err := client.Employees.Employees.List(
		context.Background(),
		&factorial.EmployeesEmployeesListParams{
			OnlyActive: true,
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	for i := range page.Data {
		employee := &page.Data[i]
		fmt.Println(value(employee.FirstName), value(employee.LastName))
	}
}

func value(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
```

Molti campi di risposta sono puntatori perché possono essere assenti o
`null`. Controllarli prima di dereferenziarli.

## Operazioni e tipi generati

In base alla risorsa possono essere disponibili:

```go
resource.List(ctx, params)
resource.Get(ctx, id)
resource.Create(ctx, body)
resource.Update(ctx, id, body)
resource.Delete(ctx, id)
resource.Paginate(ctx, params, options...)
resource.PaginatePages(ctx, params, options...)
resource.All(ctx, params, options...)
```

I nomi dei parametri e dei body seguono lo schema:

```text
<Namespace><Risorsa><Operazione>Params
<Namespace><Risorsa><Operazione>Body
```

Per esempio:

```go
params := &factorial.EmployeesEmployeesListParams{
	OnlyActive: true,
}

body := &factorial.TeamsTeamsUpdateBody{
	Description: factorial.Ptr("Team infrastruttura"),
}

team, err := client.Teams.Teams.Update(ctx, teamID, body)
```

`factorial.Ptr` è una funzione generica utile per valorizzare i campi
opzionali:

```go
factorial.Ptr("testo")
factorial.Ptr(true)
factorial.Ptr(int64(10))
```

Non tutte le operazioni esistono per tutte le risorse. Il riferimento
effettivo è `namespaces.gen.go`, mentre i modelli di risposta sono definiti in
`types.gen.go`.

## Paginazione

`List` restituisce una sola pagina:

```go
page, err := client.Employees.Employees.List(ctx, params)
```

Il risultato è un `*factorial.Page[T]` composto da:

```go
page.Data
page.Meta.EndCursor
page.Meta.HasNextPage
page.Meta.Total
```

Per elaborare progressivamente tutte le pagine:

```go
for employee, err := range client.Employees.Employees.Paginate(
	ctx,
	params,
	factorial.WithPageSize(50),
	factorial.WithMaxItems(500),
) {
	if err != nil {
		return err
	}

	// Usa employee.
}
```

Per raccogliere tutti gli elementi in una slice:

```go
employees, err := client.Employees.Employees.All(
	ctx,
	params,
	factorial.WithPageSize(50),
	factorial.WithMaxItems(500),
)
```

Opzioni:

- `WithPageSize(n)` richiede pagine da `n` elementi;
- `WithMaxItems(n)` limita il numero complessivo di elementi elaborati;
- un valore minore o uguale a zero disabilita la rispettiva opzione.

Il server Factorial limita comunque ogni pagina a un massimo di 100 elementi.

## Gestione degli errori

Le risposte HTTP non `2xx` producono un `*factorial.APIError`:

```go
_, err := client.Employees.Employees.Get(ctx, employeeID)
if err != nil {
	var apiErr *factorial.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.IsNotFound():
			// 404
		case apiErr.IsUnauthorized():
			// 401
		case apiErr.IsForbidden():
			// 403
		case apiErr.IsRateLimited():
			// 429
		default:
			// apiErr.StatusCode, apiErr.Method, apiErr.URL e apiErr.Body
		}
	}
	return err
}
```

Gli errori di rete e del transport HTTP non sono trasformati in `APIError`.
Usare sempre un `context.Context` con timeout o cancellazione appropriati
all'applicazione.

## Date e orari

I modelli generati usano:

- `factorial.Date` per giorni di calendario come `2026-07-31`;
- `factorial.Time` per timestamp RFC 3339.

Entrambi incorporano `time.Time`:

```go
date := factorial.Date{Time: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)}
timestamp := factorial.Time{Time: time.Now()}
```

La decodifica accetta anche alcune varianti di formato restituite dalle API.
La codifica produce rispettivamente `YYYY-MM-DD` e RFC 3339.

## Upload multipart

Le operazioni generate che accettano allegati usano `factorial.File`:

```go
file, err := os.Open("documento.pdf")
if err != nil {
	return err
}
defer file.Close()

upload := factorial.File{
	Name:   "documento.pdf",
	Reader: file,
}
```

Assegnare `upload` al campo `factorial.File` o `[]factorial.File` previsto
dallo specifico body generato. Il client costruisce automaticamente la
richiesta `multipart/form-data`.

## Webhook

`ParseWebhook` decodifica il body nel tipo Go associato alla subscription:

```go
payload, err := factorial.ParseWebhook(
	string(factorial.WebhookEmployeesEmployeeUpdate),
	body,
)
if err != nil {
	return err
}

employee, ok := payload.(*factorial.EmployeesEmployee)
```

Il tipo di subscription non è contenuto nella richiesta consegnata da
Factorial: deve essere noto dalla route alla quale è associata la
subscription. È quindi consigliabile usare una route distinta per ogni tipo
di evento.

Se la subscription è configurata con un challenge, verificare l'header
`x-factorial-wh-challenge` prima di elaborare il body. Gli eventi e i tipi
supportati sono elencati in `factorial.WebhookCatalog`.

## Personalizzazione del client HTTP

Per configurare timeout, proxy o transport:

```go
httpClient := &http.Client{
	Timeout: 15 * time.Second,
}

client := factorial.New(
	factorial.WithAPIKey(apiKey),
	factorial.WithHTTPClient(httpClient),
)
```

Un `http.Client` può essere condiviso e riutilizzato. Evitare di crearne uno
per ogni richiesta.

## Aggiornamento della copia

I file con suffisso `.gen.go` non devono essere modificati manualmente.
Contengono:

- i modelli delle risposte;
- namespace, risorse, parametri e operazioni;
- catalogo e parser dei webhook;
- prefisso della versione API.

Questa directory è una copia autosufficiente per l'utilizzo, ma non contiene
il generatore né la specifica OpenAPI. Per aggiornare l'SDK a una nuova
versione delle API, sostituire l'intera directory con una nuova copia generata,
preservando soltanto eventuali modifiche intenzionali apportate ai file
runtime non generati.
