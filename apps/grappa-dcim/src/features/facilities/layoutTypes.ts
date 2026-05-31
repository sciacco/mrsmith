// Shared layout selection/view types. Kept in a neutral module (not LayoutScene)
// so the 2D grid, the page and the data layer don't depend on the 3D scene file —
// the 3D view is a secondary, lazily-loaded companion to the canonical 2D map.

export type LayoutSelection =
  | { type: 'islet'; id: number }
  | { type: 'position'; id: number }
  | null;

// 'off' hides the room map (default) so the consultation list gets the full width;
// 'map' reveals the representative island canvas. (A future extruded 3D will be
// derived from the same canvas coordinates.)
export type LayoutViewMode = 'off' | 'map';
