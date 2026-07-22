import type { StatusBadgeVariant } from '@mrsmith/ui';
import type { MADomainIdentityState, MACompanyOverviewIdentity, MATarget } from '../api/types';

export type DomainIdentityPresentation = {
  domain?: string;
  state: MADomainIdentityState | 'unknown';
  label: string;
  density?: string;
  sourceLabel?: string;
  detail?: string;
  historicalNote?: string;
  variant: StatusBadgeVariant;
  source: 'registry' | 'session_validation' | 'vendor' | 'none';
};

const statePresentation: Record<MADomainIdentityState, Pick<DomainIdentityPresentation, 'label' | 'density' | 'variant'>> = {
  verified: { label: 'Identità verificata sul sito', density: 'identità verificata', variant: 'success' },
  vouched: { label: 'Dominio associato dall’analista', density: 'associato dall’analista', variant: 'accent' },
  assumed: { label: 'Associazione automatica da verificare', density: 'identità da verificare', variant: 'warning' },
};

function vendorDomain(target?: MATarget): string | undefined {
  return target?.vendorPayload?.webAndSocial?.website || target?.vendorPayload?.website;
}

function sourceLabel(method?: string, state?: MADomainIdentityState): string | undefined {
  if (method === 'manual') return 'Fornito dall’analista';
  if (method === 'auto_verified' || state === 'verified') return 'Verificato automaticamente';
  if (method) return 'Selezionato automaticamente';
  return undefined;
}

export function presentDomainIdentity(identity: MACompanyOverviewIdentity, target?: MATarget): DomainIdentityPresentation {
  const registryDomain = identity.domain?.trim();
  const validationDomain = target?.webValidation?.selectedDomain?.trim();
  const sessionState = target?.webValidation?.identityState;

  if (registryDomain) {
    const state = identity.identityState ?? 'unknown';
    const presentation = state === 'unknown'
      ? { label: 'Stato identità non disponibile', variant: 'neutral' as const, density: undefined }
      : statePresentation[state];
    return {
      domain: registryDomain,
      state,
      ...presentation,
      sourceLabel: sourceLabel(identity.domainMethod, identity.identityState),
      historicalNote: validationDomain && validationDomain !== registryDomain
        ? `Questa verifica è stata eseguita su ${validationDomain}; il dominio corrente è ${registryDomain}.`
        : undefined,
      source: 'registry',
    };
  }

  if (validationDomain) {
    const state = sessionState ?? 'unknown';
    const presentation = state === 'unknown'
      ? { label: 'Dominio analizzato in questa ricerca', variant: 'neutral' as const, density: undefined }
      : statePresentation[state];
    return {
      domain: validationDomain,
      state,
      ...presentation,
      sourceLabel: 'Dominio analizzato in questa ricerca',
      detail: 'Il dominio non è presente nel registro aziendale corrente.',
      source: 'session_validation',
    };
  }

  const vendor = vendorDomain(target)?.trim();
  if (vendor) {
    return {
      domain: vendor,
      state: 'unknown',
      label: 'Dominio indicato dalla fonte vendor',
      sourceLabel: 'Fonte vendor',
      variant: 'neutral',
      source: 'vendor',
    };
  }

  return { state: 'unknown', label: 'Stato identità non disponibile', variant: 'neutral', source: 'none' };
}
