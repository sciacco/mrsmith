// Package cpbackoffice implements the cp-backoffice mini-app backend.
//
// Routes are registered under /cp-backoffice/v1/ on the shared /api mux. Most
// routes require app_cpbackoffice_access; biometric routes also allow
// app_cpbackoffice_biometric_access.
package cpbackoffice
