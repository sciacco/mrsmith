# MrSmith Portal — React Wireframe Design Prompt

You are building a production-ready React wireframe for "MrSmith" — an internal corporate app launcher portal. The design is heavily inspired by The Matrix aesthetic. Below is a detailed specification based on the provided mockup. The result must be a complete, reusable, component-based React application.

## Visual Theme & Atmosphere

- **Background:** Near-black (#0a0a0a / #0d1117) with an animated Matrix-style digital rain (falling green characters) as a subtle full-screen background layer, slightly blurred or at low opacity so it doesn't compete with content.
- **Color palette:** Monochromatic green-on-black. Primary green: #00ff41 (Matrix green). Secondary greens: #00cc33, #33ff66 for hover/active states. Subtle dark borders using #1a3a1a or similar muted green-black.
- **Typography:** Monospace font stack for headings and the logo (e.g., "Share Tech Mono", "Fira Code", or "Courier New"). Clean sans-serif (e.g., Inter, system-ui) for descriptions/body text. All text in green shades or muted gray-green (#88aa88).
- **Effects:** Subtle green glow (box-shadow / text-shadow) on cards and headings. Optional scanline overlay or CRT flicker effect at very low opacity. Smooth hover transitions with glow intensification.

## Layout Structure

### Header / Top Bar

- **Left:** "MrSmith" logo in large bold monospace, with a small Matrix-style glyph or badge beside it (the mockup shows a small icon/symbol next to the name).
- **Right:** User avatar + name display ("Agent J. Doe") styled like a terminal user identifier. Use a simple avatar circle with green border.
- Full-width, sticky top bar with a subtle bottom border in dark green.

### Main Content Area — App Grid

- Apps are organized into **named categories** displayed as section headings (e.g., "SALES", "PURCHASE", "CUSTOMER CARE").
- Section headings: uppercase, monospace, letter-spaced, with a subtle green underline or left-border accent.
- Categories are laid out in a responsive column grid (the mockup shows ~3 columns of categories side by side on desktop).

### App Cards

Each app is represented as a card with:

1. **Icon** — a line-art/outline-style icon in Matrix green (use a consistent icon library like Lucide, Phosphor, or custom SVGs). Centered or top-aligned within the card.
2. **App Name** — bold, monospace, green text (e.g., "CRM Pro", "Pipeline Tracker", "Purchase Orders").
3. **Description** — 1-2 lines of muted green/gray text describing the app's purpose.

Card styling:

- Dark card background (#111 / #0f1a0f) with a thin green border (#1a3a1a), slightly rounded corners.
- On hover: border brightens to primary green, subtle green glow, slight scale-up or lift effect.
- Cards within a category are arranged in a horizontal row (2-3 per category row), wrapping responsively.
- Cards are clickable — each one will eventually launch its respective mini-app.

### Apps from the mockup (use these as seed data):

**SALES**

- CRM Pro — Manage client relationships, track leads, and monitor
- Pipeline Tracker — Visualize sales stages and track deals through the conversion funnel
- Quote Generator — Create, customize, and send professional sales quotes
- Vendor Connect — Maintain supplier profiles, ratings, and communication history

**PURCHASE**

- Purchase Orders — Generate and manage POs for internal and external procurement

**CUSTOMER CARE**

- Support Tickets — Log, assign, and resolve customer support requests and inquiries
- Feedback Portal — Collect, analyze, and report on customer satisfaction feedback

## Component Architecture (Reusable)

Build the following React components, each fully self-contained and reusable:

1. **`<MatrixBackground />`** — Animated falling-characters canvas background. Configurable: speed, density, character set, opacity.
2. **`<Header />`** — Logo + user info bar. Props: `appName`, `userName`, `avatarUrl`.
3. **`<AppCategory />`** — A labeled section containing a grid of app cards. Props: `title`, `apps[]`.
4. **`<AppCard />`** — Individual app launcher tile. Props: `icon`, `name`, `description`, `onClick`, `href`.
5. **`<Portal />`** — Main page composing Header + categories grid from a config/data file.
6. **`<Icon />`** — Wrapper for rendering app icons consistently (from icon library or custom SVG).

## Data Model

Apps should be defined in a separate config/data file (e.g., `apps.ts`) as an array of categories:

```ts
type App = {
  id: string;
  name: string;
  description: string;
  icon: string; // icon identifier
  href: string; // launch URL or route
};

type Category = {
  id: string;
  title: string;
  apps: App[];
};
```

## Technical Requirements

- React 18+ with TypeScript
- CSS Modules or Tailwind CSS for styling (no CSS-in-JS runtime)
- Responsive: 3 columns on desktop, 2 on tablet, 1 on mobile
- Accessible: semantic HTML, keyboard navigation, proper aria labels
- Smooth animations via CSS transitions (no heavy animation libraries)
- The Matrix rain background should use `<canvas>` for performance
- All components should be exportable and reusable independently
- Place all portal code under the `portal/` directory

## What NOT to do

- Do not use generic admin template aesthetics — this should feel cinematic and distinctive
- Do not use bright white anywhere — everything stays in the green/black spectrum
- Do not over-animate — the Matrix rain is the hero animation, cards should have subtle hover effects only
- Do not hardcode app data inside components — keep it in the config file
