# UI/UX Design System (v2)

Canonical reference for all UI work in the MrSmith monorepo. Primary consumers are LLM agents and developers building or reviewing frontend code.

**How to read this document**

- **MUST / NEVER** are hard rules — violating them is a defect. **SHOULD** is the default — deviating requires a stated reason. **MAY** is optional.
- Token *values* live in the theme CSS files (`packages/ui/src/themes/clean.css`, `packages/ui/src/themes/matrix.css`, `apps/portal/src/styles/tokens.css`). If this document and the CSS disagree, **the CSS wins** — update this document in the same change.
- Any PR that adds tokens, shared components, or new rules MUST update this document.

---

## 0. Non-negotiables

1. Mini-apps use the `clean` theme (light-only, by design — there is no dark mode for mini-apps). The portal uses `matrix`. Never mix the two languages in one surface.
2. Colors, spacing, radii, shadows, fonts, and focus rings MUST come from theme tokens. Hardcoded literals are allowed only for the documented recipes in this file (page background §4.3, entrance keyframes §8.2).
3. Base semantic colors (`--color-success`, `--color-danger`, `--color-warning`, `--color-info`) are **not text colors** — see the text-safety rules in §4.2.
4. Entrance animations fire once per navigation. NEVER re-trigger them on refetch, polling, sort, filter, or pagination (§8.3).
5. Reuse `packages/ui` components before writing new ones (§10). Mini-app icons come only from the shared `Icon` component (lucide-react).
6. Toasts confirm outcomes. They are NEVER the only surface for an error the user must act on (§14.3).
7. UI language is Italian; number/date formatting is `it-IT` (§17).

---

## 1. Design Languages

| Context | Theme | Style | Inspiration |
|---------|-------|-------|-------------|
| Portal launcher (`apps/portal`) | `matrix` | Dark, cyberpunk, immersive | The Matrix (Agent Smith) |
| Mini-apps (all other `apps/*`) | `clean` | Light, refined, data-dense | Stripe dashboard |

The portal is the dramatic entry point — digital rain, neon green, monospace type. Mini-apps switch to a professional workspace aesthetic: excellent typography, generous whitespace, restrained motion.

---

## 2. Theming Architecture

Themes activate via the `data-theme` attribute on `:root` and are defined as CSS custom properties.

| File | Theme | Used by |
|------|-------|---------|
| `packages/ui/src/themes/clean.css` | `clean` | All mini-apps |
| `packages/ui/src/themes/matrix.css` | `matrix` | Portal launcher |
| `apps/portal/src/styles/tokens.css` | Portal-specific tokens | Portal only |

**Token contract.** `matrix.css` defines only a small subset of tokens (colors + `--font-mono`); it has no spacing, radius, or shadow tokens — the portal layers its own via `tokens.css`. Shared components in `packages/ui` are designed against the **clean** token set. A shared component that must also render under `matrix` MUST verify every token it consumes is defined there, or provide a fallback value.

---

## 3. Color — Matrix Theme (Portal)

`matrix.css` (complete):

| Token | Value |
|-------|-------|
| `--color-bg` | `#0a0a0a` |
| `--color-surface` | `#111111` |
| `--color-border` | `#1a3a1a` |
| `--color-text` | `#00ff41` |
| `--color-text-muted` | `#00aa2a` |
| `--color-accent` | `#00ff41` |
| `--color-accent-hover` | `#33ff66` |
| `--font-mono` | `"Courier New", monospace` |

Portal-specific tokens (`apps/portal/src/styles/tokens.css`): `--bg-primary`, `--bg-card #0f1a0f`, `--bg-header rgba(10,10,10,0.92)`, greens (`--green-primary #00ff41`, `--green-secondary #00cc33`, `--green-hover`, `--green-muted #88aa88`, `--green-border`), glows (`--green-glow-sm/md/lg`), tints (`--green-tint`, `--green-tint-strong`), fonts (`'Share Tech Mono'` mono, `'Inter'` body), spacing `--space-xs…3xl` (0.25–2.5rem), radii (`sm 2px`, `md 6px`, `full 50%`), `--ease-default: 0.3s ease`. Matrix uses green glow `box-shadow`s instead of elevation shadows.

---

## 4. Color — Clean Theme (Mini-Apps)

### 4.1 Palette

**Backgrounds & surfaces:**
| Token | Value | Usage |
|-------|-------|-------|
| `--color-bg` | `#fafbfd` | Base page color (under the gradient, §4.3) |
| `--color-bg-elevated` | `#ffffff` | Cards, panels |
| `--color-surface` | `#f1f5f9` | Secondary surfaces, icon containers |
| `--color-surface-hover` | `#e8edf3` | Hover states |
| `--color-border` | `#cbd5e1` | Default borders, inputs |
| `--color-border-subtle` | `#e2e8f0` | Subtle dividers |

**Text:**
| Token | Value | Usage |
|-------|-------|-------|
| `--color-text` | `#0f172a` | Primary body text |
| `--color-text-secondary` | `#334155` | Secondary text, subtitles |
| `--color-text-muted` | `#475569` | Helper text, placeholders, captions |
| `--color-text-faint` | `#64748b` | Disabled and de-emphasized text |
| `--color-text-on-accent` | `#ffffff` | Text on accent-colored fills |

**Accent (indigo):**
| Token | Value | Usage |
|-------|-------|-------|
| `--color-accent` | `#635bff` | Primary actions, links |
| `--color-accent-hover` | `#5046e5` | Hover state |
| `--color-accent-subtle` | `rgba(99,91,255,0.08)` | Tinted backgrounds |
| `--color-accent-muted` | `rgba(99,91,255,0.15)` | Selected states |
| `--color-accent-glow` | `rgba(99,91,255,0.25)` | Focus rings |

**Semantic:**
| Token | Value | Usage |
|-------|-------|-------|
| `--color-success` | `#10b981` | Icons, dots, borders — not text |
| `--color-success-strong` | `#15803d` | Success **text** |
| `--color-success-bg` | `#ecfdf5` | Success backgrounds |
| `--color-success-subtle` | `rgba(16,185,129,0.10)` | Success tints |
| `--color-danger` | `#ef4444` | Icons, dots, borders — not body text |
| `--color-danger-hover` | `#dc2626` | Danger hover; also danger **text** |
| `--color-danger-subtle` | `rgba(239,68,68,0.08)` | Danger backgrounds |
| `--color-warning` | `#f59e0b` | Icons, dots, borders — not text |
| `--color-warning-strong` | `#b45309` | Warning **text** |
| `--color-warning-subtle` | `rgba(245,158,11,0.12)` | Warning backgrounds |
| `--color-info` | `#0ea5e9` | Icons, dots, borders — not text |
| `--color-info-strong` | `#075985` | Informational **text** |
| `--color-info-subtle` | `rgba(14,116,144,0.10)` | Informational backgrounds |

### 4.2 Text-safety rules (WCAG AA)

On light backgrounds (`--color-bg`, `--color-bg-elevated`, `--color-surface`):

- **Allowed as text at any size:** `--color-text`, `--color-text-secondary`, `--color-text-muted`, `--color-text-faint` (de-emphasis only), `--color-accent` (links/actions), and the strong semantics: `--color-success-strong`, `--color-warning-strong`, `--color-info-strong`, `--color-danger-hover`.
- **NEVER as running text:** `--color-success`, `--color-danger`, `--color-warning`, `--color-info`. They fail AA contrast on white. Use them for icons, status dots, borders, and fills; for text use the strong variant. Exception: bold numerals ≥ 18px MAY use the base color.
- There is no `--color-danger-strong`; use `--color-danger-hover` for danger text.

### 4.3 Page background (documented recipe)

Every clean mini-app `global.css` body MUST use:

```css
background:
  radial-gradient(circle at top left, rgba(99, 91, 255, 0.06), transparent 28%),
  linear-gradient(180deg, #f8fafc 0%, #eef2ff 100%);
```

These literals are the approved exception to token discipline. Override only after an explicit product-specific design review. (A few legacy apps — budget, compliance, cp-backoffice — predate this recipe; new apps MUST use it.)

---

## 5. Typography

### Clean theme

| Token | Value |
|-------|-------|
| `--font-sans` | `"DM Sans", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif` |
| `--font-mono` | `"JetBrains Mono", "SF Mono", "Fira Code", monospace` |

**Scale:**
| Role | Size | Weight | Extra |
|------|------|--------|-------|
| Page title | `1.75rem` (28px) | 700 | letter-spacing −0.04em |
| Section header | `0.875rem` (14px) | 600 | — |
| Form label | `0.75rem` (12px) | 600 | uppercase, letter-spacing 0.06em |
| Body | `0.9375rem` (15px) | 400 | — |
| Small | `0.8125rem` (13px) | 400 | — |
| Tiny/caption | `0.75rem` (12px) | 400 | — |
| Table header | `0.6875rem` (11px) | 600 | uppercase |

**Rules:**
- Line-height: body and table cells SHOULD use ~1.5; headings ~1.2. Never leave line-height to chance on multi-line text.
- Pages with sub-sections MAY use an intermediate heading (`1.125rem`/600) between page title and section header; keep the scale otherwise.
- **Numbers in data contexts** (tables, KPIs, money): apply `font-variant-numeric: tabular-nums`, or use `--font-mono` for dense financial grids. Never proportional figures in a numeric column.

### Matrix theme (portal)

Fonts from portal tokens: `'Share Tech Mono'` (mono), `'Inter'` (body). Neon text effect: multi-layer `text-shadow` at 10/30/60px blur with green glow.

---

## 6. Spacing, Radii, Shadows (Clean Theme)

**Spacing** — mini-app CSS MUST use this scale:
`--space-1` 4px · `--space-2` 8px · `--space-3` 12px · `--space-4` 16px · `--space-5` 20px · `--space-6` 24px · `--space-8` 32px · `--space-10` 40px · `--space-12` 48px · `--space-16` 64px

**Radii:**
`--radius-sm` 6px · `--radius-md` 10px · `--radius-lg` 14px · `--radius-xl` 20px · `--radius-2xl` 24px · `--radius-full` 999px

Use `--radius-2xl` for hero banners and large content panels; `--radius-full` for pills and badges. (Portal radii differ: 2px/6px/50% — see §3.)

**Shadows:**
| Token | Usage |
|-------|-------|
| `--shadow-xs` | Subtle depth |
| `--shadow-sm` | Cards at rest |
| `--shadow-md` | Elevated cards |
| `--shadow-lg` | Dropdowns, popovers |
| `--shadow-xl` | Modals |
| `--shadow-accent` | Accent-colored elements (primary buttons) |
| `--shadow-float` | Floating panels |

Values live in `clean.css`; never hand-write shadow literals.

---

## 7. Layering (z-index)

Convention for mini-apps (define per app as plain values; no tokens needed):

| Layer | z-index |
|-------|---------|
| Page content | 0 |
| Sticky header (AppShell) | 10 |
| Dropdowns, tooltips | 30 |
| Drawer + backdrop | 40 |
| Toasts | 60 |

`Modal` uses native `<dialog>` and renders in the browser top layer, above all z-indexes. Portal layers: MatrixBackground 0 → content 1 → ScanlineOverlay 3 → sticky header 10.

---

## 8. Motion

### 8.1 Tokens (clean theme)

| Token | Value | Usage |
|-------|-------|-------|
| `--ease-out` | `cubic-bezier(0.16, 1, 0.3, 1)` | Standard transitions |
| `--ease-spring` | `cubic-bezier(0.34, 1.56, 0.64, 1)` | Interactive/bouncy elements |
| `--duration-fast` | `150ms` | Hover, color changes |
| `--duration-normal` | `250ms` | Most transitions |
| `--duration-slow` | `400ms` | Layout shifts, panels |

Interaction transitions (hover, focus, toggles, accent bars) MUST use these tokens.

### 8.2 Entrance animations (documented recipe)

Keyframes are defined once per app in `global.css`. (Several older apps — budget, binocolo, compliance, aenad, cp-backoffice — duplicate them in page-level module CSS; do not add new duplicates, and consolidate into `global.css` when touching those files.) Literal durations below are the approved exception to token discipline.

```css
@keyframes pageEnter    { from { opacity: 0; transform: translateY(8px);   } to { opacity: 1; transform: none; } }
@keyframes rowEnter     { from { opacity: 0; transform: translateX(-12px); } to { opacity: 1; transform: none; } }
@keyframes sectionEnter { from { opacity: 0; transform: translateY(12px);  } to { opacity: 1; transform: none; } }
```

| Target | Recipe |
|--------|--------|
| `.page` wrapper | `pageEnter 0.5s var(--ease-out) both` |
| Content sections/cards | `sectionEnter 0.5s var(--ease-out) both`, staggered `animation-delay` +0.1s each |
| Table rows | `rowEnter 0.4s var(--ease-out) both`, inline `animation-delay` ≈ 40–60ms per row, **capped at ~600ms total** — rows beyond the cap share the max delay |

Other patterns: modal open 0.35s ease-spring (scale + opacity + backdrop blur); dropdown 0.25s ease-spring (scale 0.98→1, translateY −6px→0); toast 0.5s ease-spring slide-in from right; detail panel 0.4s ease-out (opacity + translateX).

### 8.3 When NOT to animate

Entrance animations exist to make **navigation** feel alive, not to decorate updates. They MUST NOT re-trigger on:

- data refetch or polling updates
- sort, filter, or pagination changes
- optimistic updates and in-place edits
- content swaps within the same route

Implementation: key the animated wrapper by route path, not by data identity; do not put entrance animation classes on components that re-mount when data changes.

### 8.4 Reduced motion

Every app's `global.css` MUST include a `prefers-reduced-motion` block disabling animations and transitions. The portal's MatrixBackground canvas also stops when reduced motion is preferred.

---

## 9. Micro-interactions

| Element | Hover | Active |
|---------|-------|--------|
| Buttons | shadow increase, `translateY(-1px)` | `scale(0.98)`, `translateY(0)` |
| Table rows | background tint, accent bar grows | `scale(0.995)` |
| Cards (clickable) | border/shadow emphasis, slight lift | — |

Disabled elements: `opacity: 0.5`, `cursor: not-allowed`, no transform or shadow.

---

## 10. Shared Components (`packages/ui`)

Reuse these before writing anything new. NEVER re-implement a listed component inside an app.

| Component | Purpose |
|-----------|---------|
| `AppShell` | Mini-app layout: sticky 60px blurred header, logo box (32×32 accent bg + name 1rem/700), compound `<AppShell.Nav>` / `<AppShell.Content>` |
| `AccessNotice` | Full-page bootstrap states (loading / access denied / error) with portal link |
| `Button` | Variants `primary \| secondary \| ghost \| danger`, sizes `sm \| md \| lg`, `loading`, `leftIcon`/`rightIcon`, `fullWidth`; visual spec in §14.2 |
| `Drawer` | Side overlay panel: `size` `sm 360px \| md 520px \| lg 720px`, `side` `left \| right`, `onDismissAttempt` to intercept/cancel close; body intentionally unpadded (rule below) |
| `Icon` | lucide-react registry behind typed `IconName` — the **only** icon source for mini-apps |
| `Modal` | Native `<dialog>` + `showModal()`; `size` `sm 400px \| md 480px (default) \| lg 640px \| wide 860px \| xwide 80vw \| fluid 90vw` (`wide` boolean prop is deprecated); mobile `100vw − 32px`; close button rotates 90° on hover |
| `MoneyInput` | Currency field; `value` is the canonical wire string (`"1500.00"`, `""` = empty); `onChange` always emits canonical; `label`/`error`/`required` |
| `MultiSelect` | Searchable multi picker (`string \| number`), chip display with remove |
| `NotificationBell` | Header bell with unread polling and per-app counts |
| `PhoneInput` | Phone field storing E.164 (`"+39333…"`), country prefix picker |
| `SearchInput` | Controlled search box, default placeholder `"Cerca..."` |
| `SingleSelect` | Searchable single picker, custom radio indicator, accent selected state |
| `Skeleton` | Shimmer loading rows (2s loop), staggered fade-in (+80ms), 48px rows, randomized widths |
| `StatusBadge` | Maps status strings to `success \| warning \| danger \| accent \| neutral`; optional dot + tooltip; ships a default Italian status map |
| `SupportMenu` | In-shell support widget: submit request, view history |
| `TableToolbar` | Table header bar: actions slot + collapsible filter area with active-filter count |
| `TabNav` | Router-aware tabs (NavLink), sliding underline indicator (400ms ease-out) |
| `TabNavGroup` | Grouped tabs with dropdown sub-navigation |
| `Toast` | Via `ToastProvider`; types `success \| error \| warning`; auto-dismiss 4s; slide-in from right |
| `ToggleSwitch` | 44×24 pill switch; `role="switch"`, hidden native checkbox; spring thumb; preferred over raw checkboxes for booleans |
| `Tooltip` | Hover/focus tooltip: `placement`, show/hide delays, `maxWidth` |
| `UserMenu` | "Agent {name}" avatar dropdown; Escape + click-outside close, `aria-expanded`/`aria-haspopup` |

**Component rules:**

- **Drawer body padding.** The shared Drawer body is intentionally unpadded so full-bleed tables, sidebars, and sticky ribbons can own their layout. Standard form/detail drawers MUST wrap children in a local body class with `padding: var(--space-5) var(--space-6) var(--space-4)` (smaller on mobile). NEVER add global body padding to the shared Drawer without auditing every consumer and adding opt-outs for full-bleed layouts.
- **Icons.** Mini-apps MUST use `Icon` (lucide). NEVER import another icon package or paste ad-hoc SVGs. The portal has its own custom SVG icon set (§11) — do not mix.
- **MoneyInput.** Application state and API payloads carry the canonical string, never a locale-formatted one.
- **Toast usage.** See §14.3.

---

## 11. Portal-Specific Components

Condensed — the portal is a single, stable app; consult its source for detail.

- **MatrixBackground** — canvas rain (hiragana + hex), speed 33, density 0.975, opacity 0.12; disabled under reduced motion.
- **ScanlineOverlay** — CRT lines, opacity 0.03, fixed, z-index 3.
- **AppCard** — badge `TEST` (amber `#ffd166`) / `READY` (neon green); hover = brighter border + glow + `translateY(-2px)`; min-height 6.25rem.
- **Portal icon set** — custom SVGs, viewBox `0 0 48 48`, 1.5px stroke, in a 36×36 tinted wrapper (26×26 render). Portal-only.
- **Header** — 1.75rem monospace logo with triple text-shadow glow; sticky with dark blurred backdrop.
- **Section titles** — terminal prompt style: `> Section Name` with blinking `_` cursor.
- **Status/Error panel** — max-width 46rem centered, gradient border + ambient glow, monospace eyebrow (0.72rem uppercase, letter-spacing 0.18em); error variant switches to red scheme.

Portal card grid: 6 cols ≥1200px, 4 ≥900px, 3 ≥640px, 2 below; gap `0.65rem`.

---

## 12. Layout Patterns (Mini-Apps)

- **Shell:** `AppShell` with sticky header, tab navigation below, content max-width **1400px** centered.
- **Page enter:** `pageEnter` on the page wrapper (§8.2).
- **Master-detail:** left scrollable list + right sticky detail panel (400px wide, top offset 92px); rows get the accent bar treatment (§13.2); stacks to a single column below **1000px**. Use it only when the detail is a quick inspection of the selected list item. When the two views need independent filters, exports, or lifecycles, build them as separate views instead — do not force master-detail.

---

## 13. Data Tables

Mini-apps are data-heavy; tables are the primary surface. Rules:

### 13.1 Structure

- Column headers: table-header type style (§5), `--color-text-muted`.
- **Numeric columns MUST be right-aligned with tabular figures** (§5). Money columns use the `it-IT` formats from §17.
- Long scrolling tables SHOULD use a sticky table header (`position: sticky; top: 0` inside the scroll container).
- Large datasets: prefer server-side pagination or windowing. NEVER entrance-stagger hundreds of rows (§8.2 cap).

### 13.2 Row interaction

- Hover: background `rgba(99,91,255,0.04)`; selected: `rgba(99,91,255,0.08)`.
- **Accent bar:** first `<td>` contains a 4px-wide div, `border-radius: 4px`, `background: var(--color-accent)`; height 0 at rest → 20px on hover → 28px selected; `transition: height var(--duration-normal) var(--ease-spring)`; selected adds `box-shadow: 0 0 8px var(--color-accent-glow)`.
- Click feedback: `scale(0.995)` via `:active`.
- **Keyboard access:** clickable rows MUST be operable without a mouse. Simplest compliant pattern: the row's primary cell contains a real `<a>`/`<button>` that triggers the same action; alternatively `tabIndex={0}` + Enter/Space handler + an appropriate role. A `<tr onClick>` alone is not acceptable.

### 13.3 Updates

Row entrance animation applies to the first render after navigation only. Refetches, polling, sort, filter, and pagination re-render **without** animation (§8.3).

---

## 14. Forms, Buttons, Feedback

### 14.1 Inputs

- Min height **44px** (touch-friendly); border 1.5px `--color-border`.
- Focus: `0 0 0 3px var(--color-accent-glow)` ring + subtle background shift, via `:focus-visible`.
- Labels: form-label type style (§5).
- **Required fields:** no literal `*`. Use a small red dot next to the label (`--color-danger`), `aria-hidden="true"`, paired with visually-hidden text `obbligatorio` (or an equivalent `aria-label`). Keep native `required` on the control; the dot is only the visual affordance.
- **Validation errors:** inline message below the field — 0.75rem, `--color-danger-hover`; set `aria-invalid="true"` and link the message with `aria-describedby`; switch the input border to danger. The message persists until the input is corrected.

### 14.2 Buttons

Use the shared `Button` (§10) — its spec is canonical:

| Variant | Style |
|---------|-------|
| `primary` | `--color-accent` background, `--color-text-on-accent` text, `--shadow-accent`; hover → `--color-accent-hover` + stronger shadow |
| `secondary` | `--color-bg-elevated` background, `--color-border` border, `--color-text`, `--shadow-xs`; hover → `--color-surface` |
| `ghost` | Transparent background and border, `--color-text-secondary` text |
| `danger` | Solid `--color-danger` background, `--color-text-on-accent` text, red-tinted shadow |

All: pill (`--radius-full`), weight 600, `inline-flex` centered; min-heights by size: `sm` 36px, `md` 44px (default), `lg` 52px; hover/active per §9; transitions on `transform`, `box-shadow`, `background` at `--duration-fast` `--ease-out`.

Legacy app-local buttons (indigo gradient `135deg → #7c6cff` primary, tinted-red danger, `2.75rem`/700) predate the shared component. Do not copy them: new code MUST use `Button`; when touching legacy buttons, migrate to it.

For destructive confirmations inside a Modal, `danger` is the primary action and `secondary`/`ghost` is the safe way out — never two primaries.

### 14.3 Toasts

- Types: `success | error | warning`; auto-dismiss 4s; color-coded background (success `rgba(16,185,129,0.92)`, error `rgba(239,68,68,0.92)`, warning `rgba(245,158,11,0.92)`); fixed top-right, 24px inset.
- Use toasts to confirm completed actions ("Salvato"). An error that requires user action MUST also appear as a persistent inline state (field error, error panel) — the toast alone is not sufficient, it disappears in 4 seconds.

### 14.4 Loading

- Skeleton screens with shimmer, not spinners; staggered row appearance for perceived speed.
- When skeletons resolve to content, do not replay entrance animations if data merely refreshed (§8.3).

### 14.5 Empty states

- Centered column layout, padding `--space-16` vertical / `--space-8` horizontal.
- Icon container 72×72px, `--radius-xl`, `--color-surface` background, `--color-text-faint` icon color, 32×32 icon, `margin-bottom: var(--space-5)`.
- Title: 0.9375rem/600, `--color-text-secondary`, letter-spacing −0.01em. Description: 0.8125rem, `--color-text-muted`, `margin-top: var(--space-1)`.
- If the user can resolve the state (create the first item, clear filters), include one primary action.
- Error empty states: same pattern with `--color-danger`-tinted icon container.

### 14.6 Status badges

Prefer `StatusBadge` (§10). Dot-style: 7×7px dot + uppercase label; success dot gets a subtle glow, disabled is gray + muted text.

---

## 15. Responsive Design

| Width | Behavior |
|-------|----------|
| ≥1200px | Full desktop layout |
| 1000–1200px | Reduced columns, condensed spacing |
| 900–1000px | Master-detail collapses to single column |
| 640–900px | Tablet: 2–3 column grids |
| <640px | Single column, full-width elements, reduced padding |

Across all sizes: sticky headers maintained; touch targets ≥44px.

---

## 16. Accessibility

Target: **WCAG 2.1 AA.**

- **Contrast:** follow the text-safety rules in §4.2.
- **Keyboard:** every interactive element operable via keyboard; Escape closes menus, drawers, modals; Enter/Space activates. Clickable table rows per §13.2.
- **Focus:** visible focus via `:focus-visible` glow rings. Because glow-only indicators disappear in forced-colors/High-Contrast mode, focus styles SHOULD also set an `outline` (a transparent outline becomes visible under forced colors).
- **Focus management:** `Modal` (native `<dialog>`) traps focus natively. `Drawer` MUST move focus into the panel on open and restore it to the trigger on close.
- **Overlays:** click-outside closes; `aria-expanded`/`aria-haspopup` on triggers; `role="alert"` on toasts.
- **Motion:** `prefers-reduced-motion` respected globally (§8.4).

---

## 17. Language & Formatting

- UI language is **Italian** — a deliberate choice, no i18n layer. Shared-component defaults ("Seleziona...", "Cerca...") are Italian.
- User greeting: `Agent {name}` in UserMenu.
- **Numbers:** `it-IT` — thousands `.`, decimals `,` (e.g. `1.234,56`). Use `Intl.NumberFormat('it-IT')`.
- **Currency:** `Intl.NumberFormat('it-IT', { style: 'currency', currency: 'EUR' })` → `1.234,56 €`. Wire/API values stay canonical (`"1234.56"`, see MoneyInput §10).
- **Dates:** `dd/mm/yyyy` via `Intl.DateTimeFormat('it-IT')`.
- Use the typographic ellipsis `…`, not `...`, in new copy.

---

## 18. CSS Architecture

- **CSS Modules** (`.module.css`), one per component, scoped class names.
- **CSS custom properties** for theming via `:root[data-theme]`.
- **No Tailwind. No CSS-in-JS.** Hand-authored CSS only.

**Token discipline:**
- App-local CSS MUST use theme tokens for colors, radii, shadows, spacing, typography, focus rings.
- If no token exists for a needed value, add or extend a token first (and update this document). The only allowed literal exceptions are the documented recipes: page background (§4.3) and entrance keyframes (§8.2).

**Naming:** component root class matches the component (`.card`, `.header`); variants in camelCase (`.cardActive`, `.badgeReady`); `@keyframes` live in the same module file.

**File structure:**
```
packages/ui/src/
  themes/            # clean.css, matrix.css
  components/
    ComponentName/
      ComponentName.tsx
      ComponentName.module.css
apps/{app}/src/
  styles/
    tokens.css       # app-specific tokens (extend theme)
    global.css       # page background, keyframes, reduced-motion block
  components/
    ComponentName/
      ComponentName.tsx
      ComponentName.module.css
```

---

## 19. New-Screen Checklist

Before shipping any new mini-app screen, verify:

1. `clean` theme, approved page background (§4.3), content max-width 1400px inside `AppShell`.
2. All values via tokens; no hardcoded colors/spacing/radii/shadows outside documented recipes.
3. Semantic colors as text only in their strong variants (§4.2).
4. Numeric/money columns right-aligned with tabular figures and `it-IT` formatting (§13.1, §17).
5. Entrance animations on navigation only; no animation replay on refetch/sort/filter/pagination (§8.3); reduced-motion block present.
6. Shared components used where they exist; icons only via `Icon` (§10).
7. Forms: 44px inputs, red-dot required marker, inline validation with `aria-invalid`/`aria-describedby` (§14.1).
8. Errors requiring action have a persistent surface, not just a toast (§14.3).
9. Loading = skeletons; empty states per §14.5 with an action when resolvable.
10. Keyboard: rows/overlays/menus operable, Escape closes, focus visible and managed (§16).
