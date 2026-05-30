// Package cpbackoffice implements the cp-backoffice mini-app backend.
//
// Routes are registered under /cp-backoffice/v1/ on the shared /api mux.
// Backoffice routes require app_cpbackoffice_access; customer state routes also
// allow app_afctools_access; biometric routes require the separate
// app_cpbackoffice_biometric_access role.
package cpbackoffice
