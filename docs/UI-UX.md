# UI/UX Design System

Reference document for all UI/UX decisions implemented in the MrSmith codebase. Use this as the foundation when building new mini-apps.

---

## 1. Design Philosophy

**Two coexisting design languages:**

| Context | Style | Inspiration |
|---------|-------|-------------|
| Portal/launcher | Dark, cyberpunk, immersive | The Matrix (Agent Smith) |
| Mini-apps (budget, etc.) | Light, clean, polished | Stripe dashboard |

The portal is the dramatic entry point — digital rain, neon green, monospace type. Individual apps switch to a refined, professional workspace aesthetic with excellent typography and whitespace.

---

## 2. Theming Architecture

Themes are activated via `data-theme` attribute on `:root` and defined as CSS custom properties.

| File | Theme | Used by |
|------|-------|---------|
| `packages/ui/src/themes/clean.css` | `clean` | Budget app and future mini-apps |
| `packages/ui/src/themes/matrix.css` | `matrix` | Portal launcher |
| `apps/portal/src/styles/tokens.css` | Portal-specific tokens | Portal only |

All components in `packages/ui/` consume theme variables, making them theme-agnostic.

---

## 3. Color Palettes

### 3.1 Clean Theme (Mini-Apps)

**Backgrounds & Surfaces:**
| Token | Value | Usage |
|-------|-------|-------|
| `--color-bg` | `#fafbfd` | Page background |
| `--color-bg-elevated` | `#ffffff` | Cards, panels |
| `--color-surface` | `#f1f5f9` | Secondary surfaces |
| `--color-surface-hover` | `#e8edf3` | Hover states |
| `--color-border` | `#e2e8f0` | Default borders |
| `--color-border-subtle` | `#f0f3f7` | Subtle dividers |

**Text:**
| Token | Value | Usage |
|-------|-------|-------|
| `--color-text` | `#0f172a` | Primary body text |
| `--color-text-secondary` | `#334155` | Secondary text |
| `--color-text-muted` | `#94a3b8` | Placeholder, helper text |
| `--color-text-faint` | `#cbd5e1` | Disabled text |

**Accent (Indigo):**
| Token | Value | Usage |
|-------|-------|-------|
| `--color-accent` | `#635bff` | Primary actions, links |
| `--color-accent-hover` | `#5046e5` | Hover state |
| `--color-accent-subtle` | `rgba(99,91,255,0.08)` | Tinted backgrounds |
| `--color-accent-muted` | `rgba(99,91,255,0.15)` | Selected states |
| `--color-accent-glow` | `rgba(99,91,255,0.25)` | Focus rings, glows |

**Semantic:**
| Token | Value | Usage |
|-------|-------|-------|
| `--color-success` | `#10b981` | Success states |
| `--color-success-bg` | `#ecfdf5` | Success backgrounds |
| `--color-danger` | `#ef4444` | Errors, destructive actions |
| `--color-danger-hover` | `#dc2626` | Danger hover |
| `--color-danger-subtle` | `rgba(239,68,68,0.08)` | Danger backgrounds |
| `--color-warning` | `#f59e0b` | Warnings |

### 3.2 Matrix Theme (Portal)

**Core:**
| Token | Value | Usage |
|-------|-------|-------|
| `--color-bg` | `#0a0a0a` | Page background |
| `--color-surface` | `#111111` | Card surfaces |
| `--color-border` | `#1a3a1a` | Green-tinted borders |
| `--color-text` | `#00ff41` | Primary neon green |
| `--color-text-muted` | `#00aa2a` | Subdued green |
| `--color-accent` | `#00ff41` | Primary accent |
| `--color-accent-hover` | `#33ff66` | Hover accent |

**Portal-Specific Tokens** (`apps/portal/src/styles/tokens.css`):
| Token | Value | Usage |
|-------|-------|-------|
| `--bg-card` | `#0f1a0f` | Card backgrounds |
| `--bg-header` | `rgba(10,10,10,0.92)` | Sticky header |
| `--green-primary` | `#00ff41` | Primary green |
| `--green-secondary` | `#00cc33` | Secondary green |
| `--green-hover` | `#33ff66` | Hover state |
| `--green-muted` | `#88aa88` | Muted text |
| `--green-glow-sm` | `rgba(0,255,65,0.3)` | Small glow |
| `--green-glow-md` | `rgba(0,255,65,0.6)` | Medium glow |
| `--green-glow-lg` | `rgba(0,255,65,0.15)` | Large ambient glow |
| `--green-tint` | `rgba(0,255,65,0.05)` | Subtle tint |
| `--green-tint-strong` | `rgba(0,255,65,0.08)` | Stronger tint |

---

## 4. Typography

### Clean Theme (Mini-Apps)
| Token | Value |
|-------|-------|
| `--font-sans` | `"DM Sans", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif` |
| `--font-mono` | `"JetBrains Mono", "SF Mono", "Fira Code", monospace` |

**Scale:**
| Role | Size | Weight | Extra |
|------|------|--------|-------|
| Page title | `1.75rem` (28px) | 700 | letter-spacing: -0.04em |
| Section header | `0.875rem` (14px) | 600 | — |
| Form label | `0.75rem` (12px) | 600 | uppercase, letter-spacing: 0.06em |
| Body | `0.9375rem` (15px) | 400 | — |
| Small | `0.8125rem` (13px) | 400 | — |
| Tiny/caption | `0.75rem` (12px) | 400 | — |
| Table header | `0.6875rem` (11px) | 600 | uppercase |

### Matrix Theme (Portal)
| Token | Value |
|-------|-------|
| `--font-mono` | `'Share Tech Mono', 'Fira Code', 'Courier New', monospace` |
| `--font-body` | `'Inter', system-ui, -apple-system, sans-serif` |

**Neon text effect:** Multi-layered `text-shadow` at 10px, 30px, 60px blur with green glow.

---

## 5. Spacing Scale

### Clean Theme
| Token | Value |
|-------|-------|
| `--space-1` | `4px` |
| `--space-2` | `8px` |
| `--space-3` | `12px` |
| `--space-4` | `16px` |
| `--space-5` | `20px` |
| `--space-6` | `24px` |
| `--space-8` | `32px` |
| `--space-10` | `40px` |
| `--space-12` | `48px` |
| `--space-16` | `64px` |

### Matrix Theme (Portal)
| Token | Value |
|-------|-------|
| `--space-xs` | `0.25rem` (4px) |
| `--space-sm` | `0.5rem` (8px) |
| `--space-md` | `0.75rem` (12px) |
| `--space-lg` | `1rem` (16px) |
| `--space-xl` | `1.25rem` (20px) |
| `--space-2xl` | `2rem` (32px) |
| `--space-3xl` | `2.5rem` (40px) |

---

## 6. Border Radius

| Token | Clean | Matrix |
|-------|-------|--------|
| `--radius-sm` | `6px` | `2px` |
| `--radius-md` | `10px` | `6px` |
| `--radius-lg` | `14px` | — |
| `--radius-xl` | `20px` | — |
| `--radius-full` | — | `50%` |

Design intent: clean theme uses generous rounding for a soft, modern feel. Matrix theme uses tighter corners for a technical, terminal look.

---

## 7. Shadows (Clean Theme)

| Token | Value | Usage |
|-------|-------|-------|
| `--shadow-xs` | `0 1px 2px rgba(15,23,42,0.04)` | Subtle depth |
| `--shadow-sm` | `0 1px 3px rgba(15,23,42,0.06), 0 1px 2px rgba(15,23,42,0.04)` | Cards at rest |
| `--shadow-md` | `0 4px 6px -1px rgba(15,23,42,0.06), 0 2px 4px -2px rgba(15,23,42,0.04)` | Elevated cards |
| `--shadow-lg` | `0 10px 15px -3px rgba(15,23,42,0.08), 0 4px 6px -4px rgba(15,23,42,0.04)` | Dropdowns, popovers |
| `--shadow-xl` | `0 20px 25px -5px rgba(15,23,42,0.08), 0 8px 10px -6px rgba(15,23,42,0.04)` | Modals |
| `--shadow-accent` | `0 4px 14px rgba(99,91,255,0.25), 0 1px 3px rgba(99,91,255,0.1)` | Accent-colored elements |
| `--shadow-float` | `0 24px 48px -12px rgba(15,23,42,0.15)` | Floating panels |

Matrix theme uses `box-shadow` glow effects with green tints instead of traditional shadows.

---

## 8. Motion & Animation

### Easing & Duration (Clean Theme)
| Token | Value | Usage |
|-------|-------|-------|
| `--ease-out` | `cubic-bezier(0.16, 1, 0.3, 1)` | Standard transitions |
| `--ease-spring` | `cubic-bezier(0.34, 1.56, 0.64, 1)` | Interactive/bouncy elements |
| `--duration-fast` | `150ms` | Hover, color changes |
| `--duration-normal` | `250ms` | Most transitions |
| `--duration-slow` | `400ms` | Layout shifts, panels |

### Animation Patterns
| Element | Duration | Easing | Effect |
|---------|----------|--------|--------|
| Page enter | 0.5s | ease-out | opacity 0→1, translateY(8px→0) |
| Row enter | 0.4s | ease-out | opacity 0→1, translateX(-12px→0), staggered |
| Modal open | 0.35s | ease-spring | scale + opacity, backdrop blur |
| Dropdown open | 0.25s | ease-spring | scale(0.98→1), translateY(-6px→0) |
| Toast | 0.5s | ease-spring | slide-in from right + scale |
| Button hover | — | — | shadow increase, translateY(-1px) |
| Button active | — | — | scale(0.98) |
| Detail panel | 0.4s | ease-out | opacity + translateX |
| Staggered children | +0.1s each | — | Sequential reveal |

### Accessibility
- All animations respect `prefers-reduced-motion` media query
- MatrixBackground canvas animation disables when reduced motion is preferred

---

## 9. Shared UI Components (`packages/ui/`)

### AppShell
- **Purpose:** Top-level layout wrapper for mini-apps
- **Structure:** Sticky header (60px) with backdrop blur + nav + content areas
- **Compound pattern:** `<AppShell.Nav>` and `<AppShell.Content>`
- **Header:** Transparent white, `backdrop-filter: blur(16px) saturate(180%)`, subtle bottom border
- **Logo:** Icon box (32x32px, accent background) + app name (1rem, weight 700)

### UserMenu
- **Display:** "Agent {name}" with custom SVG avatar or image
- **Dropdown:** Absolute positioned, fadeIn animation (150ms)
- **A11y:** Escape to close, click-outside handler, `aria-expanded`, `aria-haspopup`

### TabNav
- **Animated indicator:** Underline slides to active tab position (400ms ease-out)
- **Tab text:** 0.8125rem, color transitions 250ms
- **Uses NavLink** from react-router for active state

### Modal
- **Implementation:** Native `<dialog>` with `showModal()`
- **Animation:** Scale + opacity enter (0.35s ease-spring), backdrop blur
- **Max width:** 480px, responsive to `100vw - 48px`
- **Close button:** 32x32px circle, rotates 90deg on hover

### Toast (via ToastProvider)
- **Types:** `success` | `error`
- **Auto-dismiss:** 4 seconds
- **Animation:** Slide-in from right with spring easing
- **Position:** Fixed top-right (24px inset)
- **Colors:** Success `rgba(16,185,129,0.92)`, Error `rgba(239,68,68,0.92)`

### MultiSelect
- **Generic:** Accepts `number | string` values
- **Features:** Search filtering, chip display with remove buttons
- **Placeholder:** "Seleziona..." (Italian)
- **Focus:** 3px accent glow ring
- **Dropdown:** ease-spring animation

### SingleSelect
- **Generic:** Number values
- **Features:** Search filtering, custom radio indicator
- **Selected state:** Accent background + bold text

### Skeleton
- **Shimmer:** Gradient animation (2s infinite loop)
- **Staggered rows:** FadeIn with 80ms delay increments
- **Row height:** 48px, randomized widths (88%-100%)

---

## 10. Portal-Specific Components

### MatrixBackground
- **Canvas-based** falling character animation (Japanese hiragana + hex digits)
- **Params:** speed=33, density=0.975, opacity=0.12
- **Rendering:** Character opacity varies 0.6–1.0, fade rect opacity 0.05/frame

### ScanlineOverlay
- **CRT monitor effect:** Horizontal lines (2px transparent, 2px black)
- **Opacity:** 0.03 (very subtle)
- **Position:** Fixed, z-index 3

### AppCard
- **Badge status:** `TEST` (amber `#ffd166`) or `READY` (neon green)
- **Hover:** Green border brightens, multi-layer box-shadow glow, translateY(-2px), icon glow
- **Min height:** 6.25rem
- **Name:** 0.8rem monospace, line-clamp 2
- **Description:** 0.75rem body font

### Icon System
- **Custom SVG icons** (viewBox `0 0 48 48`, 1.5px stroke)
- **Display:** 36x36px wrapper with tinted background, 26x26px SVG
- **Available:** chart, funnel, document, handshake, cart, chat, star, coins, users, package, mail, clipboard, tag, folder, spark, shield, settings, briefcase, wrench, database, launch

### Header
- **Logo:** 1.75rem monospace, triple text-shadow glow (10px/30px/60px)
- **Glasses icon:** Inline SVG (36x16px) with glow filter (4px blur)
- **Sticky** with dark backdrop + blur

### Section Titles
- Terminal prompt style: `> Section Name` with blinking cursor `_`

### Status Panel
- **Max width:** 46rem, centered
- **Styling:** Gradient border + ambient glow, monospace eyebrow label
- **Error variant:** Red theme with red gradients/glows

---

## 11. Layout Patterns

### Portal Layout
| Layer | z-index | Element |
|-------|---------|---------|
| 0 | 0 | MatrixBackground canvas |
| 1 | 1 | Content wrapper |
| 2 | 3 | ScanlineOverlay |
| 3 | 10 | Sticky header |

**Card Grid:**
| Breakpoint | Columns |
|------------|---------|
| ≥1200px | 6 |
| ≥900px | 4 |
| ≥640px | 3 |
| <640px | 2 |

Grid gap: `0.65rem`

### Mini-App Layout (Budget)
- **AppShell** wrapper with sticky header
- **Tab navigation** below header
- **Content area:** max-width 1400px, centered with auto margins
- **Page-level** enter animations

### Master-Detail Pattern (Budget)
- **Left column:** Scrollable list with selectable rows
- **Right column:** Sticky detail panel (400px, top offset 92px)
- **Row states:** Accent bar (4px→28px on hover/select), icon background change
- **Responsive:** Stacks below 1000px breakpoint

---

## 12. Interactive Patterns

### Buttons
| Variant | Style | Hover | Active |
|---------|-------|-------|--------|
| Primary | Indigo bg, white text, uppercase, weight 600 | Darker indigo, larger shadow, translateY(-1px) | scale(0.98) |
| Secondary | Border, light bg | Tinted bg, darker border, shadow | — |
| Danger | Red text, red border | Inverted (white on red) | — |

All buttons: disabled state at `opacity: 0.5`, no pointer events.

### Form Inputs
- **Height:** 44px minimum (touch-friendly)
- **Border:** 1.5px, transitions on focus
- **Focus:** 3px accent glow ring + background shift
- **Labels:** 0.75rem uppercase, weight 600, letter-spacing 0.06em

### Toggle Switch
- **Size:** 44x24px
- **Animation:** Thumb translates 20px on active

### Slider (Threshold)
- **Thumb:** 20px, hover scale(1.15)
- **Custom styled** track and thumb

### Table Rows
- **Hover:** Background tint, accent bar height animation
- **Selected:** Accent color styling, icon background change
- **Click:** scale(0.995) micro-feedback
- **Accent bar:** Left edge indicator, 4px default → 28px on hover/select

---

## 13. Status & Feedback

### Badges
| Type | Color | Style |
|------|-------|-------|
| TEST | `#ffd166` (amber) | Translucent background, uppercase, 0.55rem |
| READY | Neon green | Same pattern |
| Success | Green dot (7x7px) + glow | Uppercase text |
| Disabled | Gray dot | Muted text |

### Loading States
- **Skeleton screens** with shimmer animation (not spinners)
- Staggered row appearance for perceived speed

### Toast Notifications
- Color-coded (green/red) with matching box-shadow
- Auto-dismiss after 4 seconds
- Spring-animated slide from right

### Empty States
- Centered layout with icon container (72x72px rounded)
- Title (0.9375rem, weight 600) + description (0.8125rem, muted)

### Error/Status Panels (Portal)
- Gradient border + ambient glow
- Monospace eyebrow label (0.72rem, uppercase, letter-spacing 0.18em)
- Error variant switches to red gradient scheme

---

## 14. Responsive Design

### Breakpoint Strategy
| Width | Behavior |
|-------|----------|
| ≥1200px | Full desktop layout |
| 900–1200px | Reduced columns, condensed spacing |
| 640–900px | Tablet-friendly, 2-3 column grids |
| <640px | Single column stacked, full-width elements |

### Mobile Adaptations
- Reduced padding (e.g., `--space-lg` instead of `--space-2xl`)
- Sticky headers maintained across all sizes
- Touch targets minimum 44px height
- Master-detail collapses to single column below 1000px

---

## 15. Accessibility

- **ARIA attributes:** `role="menu"`, `role="alert"`, `aria-label`, `aria-expanded`, `aria-haspopup`
- **Keyboard:** Escape closes menus/modals, Enter/Space activates buttons
- **Click-outside:** All overlays close on outside click
- **prefers-reduced-motion:** Animations disabled when requested
- **Color contrast:** High contrast in both themes
- **Focus indicators:** Glow rings on interactive elements
- **Native `<dialog>`:** Used for modals (proper focus trapping)

---

## 16. CSS Architecture

### Styling Approach
- **CSS Modules** (`.module.css`) — one per component, scoped class names
- **CSS Custom Properties** — theming via `:root[data-theme]`
- **No Tailwind** — all styles are hand-authored CSS
- **No CSS-in-JS** — pure CSS with module scoping

### Naming Conventions
- Component root class matches component name (e.g., `.card`, `.header`)
- Variant classes use camelCase (e.g., `.cardActive`, `.badgeReady`)
- Animations defined as `@keyframes` in the same module file

### File Structure
```
packages/ui/src/
  themes/           # Theme variable definitions
    clean.css
    matrix.css
  components/       # Shared components
    ComponentName/
      ComponentName.tsx
      ComponentName.module.css
apps/{app}/src/
  styles/
    tokens.css      # App-specific tokens (extend theme)
    global.css      # App-specific globals
  components/       # App-specific components
    ComponentName/
      ComponentName.tsx
      ComponentName.module.css
```

---

## 17. Language & Localization

- Default UI language: **Italian** (placeholder text "Seleziona...", etc.)
- User greeting: "Agent {name}" format in UserMenu
