// Package cpbackoffice implements the cp-backoffice mini-app backend.
//
// Routes are registered under /cp-backoffice/v1/ on the shared /api mux and
// gated by the app_cpbackoffice_access role. See apps/customer-portal/FINAL.md
// (§Slice S2) for the contract.
package cpbackoffice
