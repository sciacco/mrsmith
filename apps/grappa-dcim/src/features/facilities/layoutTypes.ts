// Shared layout selection/view types. Kept in a neutral module (not LayoutScene)
// so the 2D grid, the page and the data layer don't depend on the 3D scene file —
// the 3D view is a secondary, lazily-loaded companion to the canonical 2D map.

export type LayoutSelection =
  | { type: 'islet'; id: number }
  | { type: 'position'; id: number }
  | null;

export type LayoutViewMode = '2d' | '3d';
