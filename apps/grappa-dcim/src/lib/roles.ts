export const GRAPPA_DCIM_VIEWER_ROLE = 'app_grappadcim_viewer';
export const GRAPPA_DCIM_OPERATIVO_ROLE = 'app_grappadcim_operativo';

export const GRAPPA_DCIM_ACCESS_ROLES = [
  GRAPPA_DCIM_VIEWER_ROLE,
  GRAPPA_DCIM_OPERATIVO_ROLE,
] as const;

export const GRAPPA_DCIM_OPERATIVE_ROLES = [GRAPPA_DCIM_OPERATIVO_ROLE] as const;
