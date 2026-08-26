package training

import (
	"context"

	"github.com/sciacco/mrsmith/pkg/factorial"
)

// Lettura paginata del grafo formativo Factorial (#141, slice 3/8 = #145):
// solo letture, nessuna scrittura verso Factorial ne' verso il database.
// fetchFactorialTrainingGraph legge integralmente le sei risorse trainings
// con i metodi .All(ctx, params) del client, stesso pattern di
// factorial_diag.go. Qualsiasi errore di fetch abortisce la run: mai
// lavorare su un grafo parziale (le funzioni pure di
// factorial_sync_perimeter.go assumono un grafo completo).

// factorialTrainingGraph e' l'istantanea completa e non filtrata del grafo
// formativo Factorial letta per una run di sincronizzazione.
type factorialTrainingGraph struct {
	Trainings                []factorial.TrainingsTraining
	TrainingClasses          []factorial.TrainingsTrainingClass
	Sessions                 []factorial.TrainingsSession
	TrainingMemberships      []factorial.TrainingsTrainingMembership
	SessionAccessMemberships []factorial.TrainingsSessionAccessMembership
	SessionAttendances       []factorial.TrainingsSessionAttendance
}

// fetchFactorialTrainingGraph legge trainings, classi, sessioni, training
// membership e attendance con una singola chiamata .All ciascuna. La
// risorsa session_access_memberships richiede un session_id per chiamata
// (campo obbligatorio nel client, non opzionale): si itera sulle sessioni
// appena lette, una chiamata .All per sessione.
func fetchFactorialTrainingGraph(ctx context.Context, cli *factorial.Client) (factorialTrainingGraph, error) {
	trainings, err := cli.Trainings.Trainings.All(ctx, &factorial.TrainingsTrainingsListParams{})
	if err != nil {
		return factorialTrainingGraph{}, err
	}
	classes, err := cli.Trainings.TrainingClasses.All(ctx, &factorial.TrainingsTrainingClassesListParams{})
	if err != nil {
		return factorialTrainingGraph{}, err
	}
	sessions, err := cli.Trainings.Sessions.All(ctx, &factorial.TrainingsSessionsListParams{})
	if err != nil {
		return factorialTrainingGraph{}, err
	}
	// TrainingsTrainingMembershipsListParams.DueDate e' factorial.Date, non
	// puntatore: encode() lo aggiunge sempre senza guardia (a differenza di
	// ogni altro filtro di questa struct), quindi questa chiamata invia
	// sempre due_date=0001-01-01 anche con params altrimenti vuoti. Se il
	// server Factorial filtrasse le training membership su quella data,
	// questa lettura "globale" sarebbe inaffidabile (rischio di escludere
	// membership con una due_date reale diversa dal valore zero). Verifica
	// live del comportamento reale del server e' fuori scope in questa
	// slice (vietate chiamate Factorial live): l'esito verra' ratificato
	// prima della slice 4, che consuma questo grafo.
	memberships, err := cli.Trainings.TrainingMemberships.All(ctx, &factorial.TrainingsTrainingMembershipsListParams{})
	if err != nil {
		return factorialTrainingGraph{}, err
	}
	var access []factorial.TrainingsSessionAccessMembership
	for _, s := range sessions {
		if s.ID == nil {
			continue
		}
		perSession, err := cli.Trainings.SessionAccessMemberships.All(ctx, &factorial.TrainingsSessionAccessMembershipsListParams{SessionID: *s.ID})
		if err != nil {
			return factorialTrainingGraph{}, err
		}
		access = append(access, perSession...)
	}
	attendances, err := cli.Trainings.SessionAttendances.All(ctx, &factorial.TrainingsSessionAttendancesListParams{})
	if err != nil {
		return factorialTrainingGraph{}, err
	}
	return factorialTrainingGraph{
		Trainings:                trainings,
		TrainingClasses:          classes,
		Sessions:                 sessions,
		TrainingMemberships:      memberships,
		SessionAccessMemberships: access,
		SessionAttendances:       attendances,
	}, nil
}
