import test from 'node:test';
import assert from 'node:assert/strict';
import {
  APP_ACCESS_ROLES,
  CP_BACKOFFICE_FULL_ACCESS_ROLES,
  getAppAccessState,
  hasAnyRole,
} from '../src/roles.ts';

function auth(overrides: Partial<Parameters<typeof getAppAccessState>[0]> = {}) {
  return {
    status: 'authenticated',
    authenticated: true,
    loading: false,
    user: { roles: [] },
    ...overrides,
  } satisfies Parameters<typeof getAppAccessState>[0];
}

test('missing required app role is forbidden', () => {
  assert.equal(
    getAppAccessState(auth({ user: { roles: ['viewer'] } }), ['app_budget_access']),
    'forbidden',
  );
});

test('matching required app role is allowed', () => {
  assert.equal(
    getAppAccessState(auth({ user: { roles: ['app_budget_access'] } }), ['app_budget_access']),
    'allowed',
  );
});

test('cp backoffice biometric role grants app access but not full access', () => {
  const roles = ['app_cpbackoffice_biometric_access'];

  assert.equal(
    getAppAccessState(auth({ user: { roles } }), APP_ACCESS_ROLES['cp-backoffice']),
    'allowed',
  );
  assert.equal(hasAnyRole(roles, CP_BACKOFFICE_FULL_ACCESS_ROLES), false);
});

test('training people role grants app access', () => {
  assert.equal(
    getAppAccessState(auth({ user: { roles: ['app_training_people_admin'] } }), APP_ACCESS_ROLES.training),
    'allowed',
  );
});

test('app_devadmin bypass is allowed', () => {
  assert.equal(
    getAppAccessState(auth({ user: { roles: ['app_devadmin'] } }), ['app_budget_access']),
    'allowed',
  );
});

test('unauthenticated user is unauthenticated', () => {
  assert.equal(
    getAppAccessState(auth({ status: 'unauthenticated', authenticated: false, user: null }), [
      'app_budget_access',
    ]),
    'unauthenticated',
  );
});

test('loading auth state is loading', () => {
  assert.equal(
    getAppAccessState(auth({ status: 'loading', authenticated: false, loading: true, user: null }), [
      'app_budget_access',
    ]),
    'loading',
  );
});

test('reauthenticating auth state is reauthenticating', () => {
  assert.equal(
    getAppAccessState(
      auth({ status: 'reauthenticating', authenticated: false, user: { roles: ['viewer'] } }),
      ['app_budget_access'],
    ),
    'reauthenticating',
  );
});
