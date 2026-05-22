import type {
  CustomGroup,
  MandatoryRule,
  PersonFlagKey,
  PersonFlags,
  PersonSummary,
  WorkspaceResponse,
} from './types';

interface MockPeopleDirectoryFilters {
  team?: string;
  group?: string;
  filter?: string;
  q?: string;
}

const EMPTY_FLAGS: PersonFlags = {
  da_pianificare: false,
  compliance_gap: false,
  scadenze_imminenti: false,
  failed_recente: false,
  senza_formazione_attiva: false,
};

function flags(overrides: Partial<PersonFlags>): PersonFlags {
  return { ...EMPTY_FLAGS, ...overrides };
}

const mockDirectoryRows: PersonSummary[] = [
  {
    id: 'employee-ada',
    name: 'Verdi Ada',
    email: 'ada.verdi@cdlan.it',
    team_code: 'CLOUD',
    flags: flags({ da_pianificare: true, compliance_gap: true }),
    active_enrollments_count: 1,
    next_deadline: null,
    priority_score: 5010,
    gaps_open: 1,
    expiring_certs_count: 0,
    historical_enrollments: 4,
  },
  {
    id: 'employee-marco',
    name: 'Rossi Marco',
    email: 'marco.rossi@cdlan.it',
    team_code: 'APPLICATIONS',
    flags: flags({ compliance_gap: true }),
    active_enrollments_count: 2,
    next_deadline: null,
    priority_score: 4010,
    gaps_open: 1,
    expiring_certs_count: 0,
    historical_enrollments: 8,
  },
  {
    id: 'employee-laura',
    name: 'Bianchi Laura',
    email: 'laura.bianchi@cdlan.it',
    team_code: 'CLOUD',
    flags: flags({ scadenze_imminenti: true }),
    active_enrollments_count: 1,
    next_deadline: { type: 'cert', date: '2026-06-19', label: 'Cert in scadenza' },
    priority_score: 3005,
    gaps_open: 0,
    expiring_certs_count: 1,
    historical_enrollments: 6,
  },
  {
    id: 'employee-federico',
    name: 'Neri Federico',
    email: 'federico.neri@cdlan.it',
    team_code: 'SECURITY',
    flags: flags({ failed_recente: true }),
    active_enrollments_count: 1,
    next_deadline: null,
    priority_score: 2503,
    gaps_open: 0,
    expiring_certs_count: 0,
    historical_enrollments: 5,
  },
  {
    id: 'employee-giulia',
    name: 'Gallo Giulia',
    email: 'giulia.gallo@cdlan.it',
    team_code: 'PEOPLE',
    flags: flags({ senza_formazione_attiva: true }),
    active_enrollments_count: 0,
    next_deadline: null,
    priority_score: 1001,
    gaps_open: 0,
    expiring_certs_count: 0,
    historical_enrollments: 2,
  },
  {
    id: 'employee-marta',
    name: 'Conti Marta',
    email: 'marta.conti@cdlan.it',
    team_code: 'APPLICATIONS',
    flags: flags({ da_pianificare: true, compliance_gap: true, scadenze_imminenti: true }),
    active_enrollments_count: 0,
    next_deadline: { type: 'mandatory_due', date: '2026-06-04', label: 'Ricorrenza obbligatoria' },
    priority_score: 5016,
    gaps_open: 1,
    expiring_certs_count: 1,
    historical_enrollments: 3,
  },
  {
    id: 'employee-nadia',
    name: 'Ferri Nadia',
    email: 'nadia.ferri@cdlan.it',
    team_code: 'FINANCE',
    flags: flags({}),
    active_enrollments_count: 2,
    next_deadline: { type: 'course_end', date: '2026-07-10', label: 'Fine corso prevista' },
    priority_score: 0,
    gaps_open: 0,
    expiring_certs_count: 0,
    historical_enrollments: 9,
  },
];

function filterMatches(row: PersonSummary, filter: string): boolean {
  if (!filter) return true;
  if (filter in row.flags) return row.flags[filter as PersonFlagKey];
  return true;
}

export function mockPeopleDirectory(filters: MockPeopleDirectoryFilters = {}): PersonSummary[] {
  const team = filters.team?.trim() ?? '';
  const group = filters.group?.trim() ?? '';
  const q = filters.q?.trim().toLowerCase() ?? '';
  const filter = filters.filter?.trim() ?? '';
  const groupMembers = group
    ? new Set((mockCustomGroups().find((item) => item.id === group)?.members ?? []).map((member) => member.id))
    : null;

  return mockDirectoryRows.filter((row) => {
    if (team && row.team_code !== team) return false;
    if (groupMembers && !groupMembers.has(row.id)) return false;
    if (q && !`${row.name} ${row.email}`.toLowerCase().includes(q)) return false;
    return filterMatches(row, filter);
  });
}

export function mockCustomGroups(): CustomGroup[] {
  return [
    {
      id: 'group-cyber-2026',
      name: 'Cybersecurity 2026',
      description: 'Popolazione trasversale per aggiornamento sicurezza',
      active: true,
      member_count: 3,
      members: [
        { id: 'employee-ada', name: 'Verdi Ada', email: 'ada.verdi@cdlan.it', team_code: 'CLOUD' },
        { id: 'employee-marco', name: 'Rossi Marco', email: 'marco.rossi@cdlan.it', team_code: 'APPLICATIONS' },
        { id: 'employee-marta', name: 'Conti Marta', email: 'marta.conti@cdlan.it', team_code: 'APPLICATIONS' },
      ],
      used_by: [{ kind: 'rule', id: 'rule-security', label: 'Sicurezza applicativa' }],
    },
    {
      id: 'group-cloud-leads',
      name: 'Cloud leads',
      description: 'Referenti operativi Cloud',
      active: true,
      member_count: 2,
      members: [
        { id: 'employee-ada', name: 'Verdi Ada', email: 'ada.verdi@cdlan.it', team_code: 'CLOUD' },
        { id: 'employee-laura', name: 'Bianchi Laura', email: 'laura.bianchi@cdlan.it', team_code: 'CLOUD' },
      ],
      used_by: [],
    },
  ];
}

export function mockMandatoryRules(): MandatoryRule[] {
  return [
    {
      id: 'rule-security',
      name: 'Sicurezza applicativa',
      course_id: 'course-2',
      course_title: 'Sicurezza applicativa OWASP',
      compliance_framework: 'ISO 27001',
      cadence_label: '12 mesi',
      population_target: {
        kind: 'custom_group',
        id: 'group-cyber-2026',
        label: 'Cybersecurity 2026',
        count: 3,
      },
      active: true,
      coverage_pct: 33,
      covered_count: 1,
      target_count: 3,
      gap_count: 2,
      gaps: [
        { employee_id: 'employee-ada', employee_name: 'Verdi Ada', status: 'never_covered' },
        { employee_id: 'employee-marta', employee_name: 'Conti Marta', status: 'expired', detail: '2025-03-12' },
      ],
      severity: 'critical',
      used_by: [{ kind: 'enrollment', label: 'Iscrizioni collegate', count: 4 }],
    },
    {
      id: 'rule-cloud',
      name: 'Kubernetes base',
      course_id: 'course-1',
      course_title: 'Certified Kubernetes Administrator',
      cadence_label: 'una tantum',
      population_target: {
        kind: 'team',
        id: 'team-cloud',
        label: 'CLOUD - Cloud Operations',
        count: 2,
      },
      active: true,
      coverage_pct: 100,
      covered_count: 2,
      target_count: 2,
      gap_count: 0,
      gaps: [],
      severity: 'ok',
      used_by: [],
    },
  ];
}

export function mockWorkspace(isPeopleAdmin: boolean): WorkspaceResponse {
  const me = {
    principal: {
      subject: 'dev-user-001',
      email: 'john.doe@acme.com',
      name: 'John Doe',
      roles: isPeopleAdmin
        ? ['app_training_access', 'app_training_people_admin']
        : ['app_training_access'],
      isPeopleAdmin,
    },
    employee: {
      id: 'employee-dev',
      firstName: 'John',
      lastName: 'Doe',
      email: 'john.doe@acme.com',
      status: 'active',
    },
    onboardingPending: false,
  };

  const plan = [
    {
      id: 'enr-1',
      employeeName: 'Bianchi Laura',
      employeeEmail: 'laura.bianchi@cdlan.it',
      teamCode: 'CLOUD',
      courseTitle: 'Certified Kubernetes Administrator',
      vendorName: 'Linux Foundation',
      skillAreaName: 'Kubernetes',
      status: 'approved',
      year: 2026,
      priority: 2,
      levelAsIs: 2,
      levelToBe: 4,
      plannedStart: '2026-06-15',
      plannedEnd: '2026-06-19',
      hoursPlanned: 32,
      costPlanned: 1290,
      objective: 'Preparare il team alla gestione autonoma dei cluster.',
      documentId: 'enr-doc-1',
      documentFilename: 'iscrizione-cka.pdf',
      documentValidated: true,
      mandatory: false,
    },
    {
      id: 'enr-2',
      employeeName: 'Rossi Marco',
      employeeEmail: 'marco.rossi@cdlan.it',
      teamCode: 'APPLICATIONS',
      courseTitle: 'Sicurezza applicativa OWASP',
      vendorName: 'Internal Academy',
      skillAreaName: 'Security',
      status: 'in_progress',
      year: 2026,
      priority: 1,
      plannedStart: '2026-05-04',
      plannedEnd: '2026-05-18',
      hoursPlanned: 12,
      costPlanned: 0,
      motivation: 'Percorso obbligatorio per il presidio sicurezza.',
      documentValidated: false,
      mandatory: true,
    },
    {
      id: 'enr-3',
      employeeName: 'John Doe',
      employeeEmail: 'john.doe@acme.com',
      teamCode: 'PEOPLE',
      courseTitle: 'Gestione colloqui e feedback',
      vendorName: 'POLIMI GSoM',
      skillAreaName: 'People',
      status: 'proposed',
      year: 2026,
      priority: 3,
      plannedStart: '2026-09-08',
      plannedEnd: '2026-09-09',
      hoursPlanned: 14,
      costPlanned: 780,
      notes: 'Da confermare con il responsabile People.',
      documentValidated: false,
      mandatory: false,
    },
  ];

  const visiblePlan = isPeopleAdmin ? plan : plan.filter((item) => item.employeeEmail === me.principal.email);

  return {
    me,
    plan: visiblePlan,
    requests: [
      {
        id: 'req-1',
        employeeName: 'John Doe',
        employeeEmail: 'john.doe@acme.com',
        freeTextTitle: 'Approfondimento su People analytics',
        skillAreaName: 'People',
        motivation: 'Preparare report periodici e analisi per il team People.',
        desiredYear: 2026,
        status: 'submitted',
        createdAt: '2026-05-12T09:00:00Z',
      },
      {
        id: 'req-2',
        employeeName: 'Rossi Marco',
        employeeEmail: 'marco.rossi@cdlan.it',
        courseId: 'course-3',
        courseTitle: 'Terraform Associate',
        skillAreaName: 'Infrastructure as Code',
        motivation: 'Allineare le competenze del team sulle nuove automazioni.',
        desiredYear: 2026,
        status: 'accepted',
        createdAt: '2026-05-10T14:30:00Z',
      },
    ].filter((item) => isPeopleAdmin || item.employeeEmail === me.principal.email),
    catalog: [
      {
        id: 'course-1',
        title: 'Certified Kubernetes Administrator',
        vendorId: 'vendor-linux',
        vendorName: 'Linux Foundation',
        skillAreaId: 'skill-kubernetes',
        skillAreaName: 'Kubernetes',
        leadsToCertId: 'cert-cka',
        certificationName: 'CKA',
        deliveryMode: 'online_live',
        providerKind: 'external',
        defaultHours: 32,
        defaultCost: 1290,
        courseUrl: 'https://training.linuxfoundation.org/',
        description: 'Percorso certificazione Kubernetes per amministratori.',
        mandatory: false,
        active: true,
      },
      {
        id: 'course-3',
        title: 'Terraform Associate',
        vendorId: 'vendor-hashicorp',
        vendorName: 'HashiCorp',
        skillAreaId: 'skill-iac',
        skillAreaName: 'Infrastructure as Code',
        leadsToCertId: 'cert-terraform',
        certificationName: 'Terraform Associate',
        deliveryMode: 'online_self',
        providerKind: 'external',
        defaultHours: 16,
        defaultCost: 70,
        courseUrl: 'https://developer.hashicorp.com/terraform',
        description: 'Preparazione alla certificazione Terraform Associate.',
        mandatory: false,
        active: true,
      },
      {
        id: 'course-2',
        title: 'Sicurezza applicativa OWASP',
        vendorId: 'vendor-internal',
        vendorName: 'Internal Academy',
        skillAreaId: 'skill-security',
        skillAreaName: 'Security',
        deliveryMode: 'mixed',
        providerKind: 'internal',
        defaultHours: 12,
        defaultCost: 0,
        description: 'Formazione ricorrente per sviluppo sicuro.',
        mandatory: true,
        recurrenceMonths: 12,
        complianceFramework: 'ISO 27001',
        active: true,
      },
    ],
    certifications: [
      {
        awardId: 'award-1',
        employeeName: 'John Doe',
        employeeEmail: 'john.doe@acme.com',
        certificationCode: 'ITIL4',
        certificationName: 'ITIL 4 Foundation',
        outcome: 'passed_exam',
        awardedOn: '2024-09-18',
        expiresOn: '2027-09-18',
        currentStatus: 'valid',
        validationSource: 'document_verified',
        documentId: 'doc-1',
        documentFilename: 'itil-foundation.pdf',
        documentValidated: true,
      },
      {
        awardId: 'award-2',
        employeeName: 'Bianchi Laura',
        employeeEmail: 'laura.bianchi@cdlan.it',
        certificationCode: 'CKA',
        certificationName: 'Certified Kubernetes Administrator',
        outcome: 'passed_exam',
        awardedOn: '2026-05-01',
        currentStatus: 'valid',
        validationSource: 'imported_legacy',
        documentId: 'doc-2',
        documentFilename: 'cka-attestato.pdf',
        documentValidated: false,
      },
    ].filter((item) => isPeopleAdmin || item.employeeEmail === me.principal.email),
    planBudget: [
      { year: 2026, teamCode: 'APPLICATIONS', enrollmentsCount: 18, costTotal: 18400, hoursTotal: 210 },
      { year: 2026, teamCode: 'CLOUD', enrollmentsCount: 12, costTotal: 16320, hoursTotal: 176 },
      { year: 2026, teamCode: 'PEOPLE', enrollmentsCount: 4, costTotal: 3120, hoursTotal: 44 },
    ],
    expiringCertifications: [
      {
        employeeName: 'John Doe',
        employeeEmail: 'john.doe@acme.com',
        certificationCode: 'ITIL4',
        certificationName: 'ITIL 4 Foundation',
        expiresOn: '2027-09-18',
        daysToExpiry: 486,
      },
    ],
    mandatoryComplianceGaps: [
      {
        employeeName: 'Rossi Marco',
        courseTitle: 'Sicurezza applicativa OWASP',
        complianceFramework: 'ISO 27001',
        complianceStatus: 'missing_or_expired',
      },
    ],
    masterData: isPeopleAdmin
      ? {
          vendors: [
            { id: 'vendor-linux', name: 'Linux Foundation', website: 'https://training.linuxfoundation.org/', active: true },
            { id: 'vendor-hashicorp', name: 'HashiCorp', website: 'https://developer.hashicorp.com/', active: true },
            { id: 'vendor-internal', name: 'Internal Academy', notes: 'Percorsi gestiti internamente', active: true },
          ],
          teams: [
            { id: 'team-cloud', code: 'CLOUD', name: 'Cloud Operations', active: true },
            { id: 'team-app', code: 'APPLICATIONS', name: 'Applications', active: true },
          ],
          skillAreas: [
            { id: 'skill-kubernetes', code: 'K8S', name: 'Kubernetes', active: true },
            { id: 'skill-iac', code: 'IAC', name: 'Infrastructure as Code', active: true },
            { id: 'skill-security', code: 'SEC', name: 'Security', active: true },
          ],
          certifications: [
            { id: 'cert-itil4', code: 'ITIL4', name: 'ITIL 4 Foundation', active: true },
            { id: 'cert-cka', code: 'CKA', name: 'Certified Kubernetes Administrator', issuerVendorId: 'vendor-linux', issuerVendorName: 'Linux Foundation', skillAreaId: 'skill-kubernetes', skillAreaLabel: 'K8S - Kubernetes', typicalValidityMonths: 36, active: true },
            { id: 'cert-terraform', code: 'TF-A', name: 'Terraform Associate', issuerVendorId: 'vendor-hashicorp', issuerVendorName: 'HashiCorp', skillAreaId: 'skill-iac', skillAreaLabel: 'IAC - Infrastructure as Code', active: true },
          ],
          plans: [
            { id: 'plan-2026', year: 2026, status: 'open', budgetTotal: 37840 },
            { id: 'plan-2025', year: 2025, status: 'closed', budgetTotal: 24500, notes: 'Storico importato' },
          ],
          mandatoryRules: [
            { id: 'rule-security', courseId: 'course-2', courseTitle: 'Sicurezza applicativa OWASP', teamId: 'team-app', teamLabel: 'APPLICATIONS - Applications', roleFilter: 'developer', active: true },
          ],
        }
      : undefined,
  };
}
