---
name: tintoretto
description: Self-contained design-execution skill for MrSmith mini-apps. Use for any scoped UI/frontend design or styling work under apps/ — new screens, restyling, shared-component usage, tables, forms, drawers, empty states. Applies docs/UI-UX.md as the canonical design system, embeds the visual-design and copy craft (no external skill dependencies), and finishes with type-check + UI smoke test. Not for whole-app planning (portal-miniapp-generator), blocking approval (portal-miniapp-ui-review), or full-app remediation (portal-miniapp-ui-fixer).
user-invocable: true
allowed-tools: Read Grep Glob Bash Edit Write
---

# Tintoretto — design execution for MrSmith mini-apps

Self-contained design skill for scoped UI work. **System rules** live in `docs/UI-UX.md`; **design craft** lives here. If this file and that document ever disagree, `docs/UI-UX.md` wins.

## Rule sources

1. **`docs/UI-UX.md`** — canonical design system. Read it before designing anything; do not work from memory of it. Key anchors: §0 non-negotiables, §4.2 text-safety, §8.3 when-not-to-animate, §10 shared components, §13 data tables, §19 new-screen checklist.
2. **The mini-app family skills** — for planning a whole new mini-app use `portal-miniapp-generator`; for blocking UI approval use `portal-miniapp-ui-review`; for full-app UI remediation use `portal-miniapp-ui-fixer`. Tintoretto complements them for scoped work and must not replicate their archetype or approval rules.

## Workflow

1. **Read `docs/UI-UX.md`** (at minimum §0 and the sections relevant to the task).
2. **Recon before invention.** Look at 1–2 comparable screens in sibling apps (`apps/*`) and at `packages/ui` before designing anything new. If a shared component or an established pattern covers the need, use it.
3. **Plan, then critique the plan** (see *Design craft* below). For routine work — wiring a form, adding a table column — the design system alone is enough; skip straight to implementation.
4. **Implement** with token discipline: CSS Modules, theme tokens only, documented recipes for the two allowed literal exceptions (page background, entrance keyframes).
5. **Verify** — both are mandatory:
   - `pnpm --filter <app> exec tsc --noEmit` (never bare `npx tsc`).
   - UI smoke test in a real browser: reuse the already-running dev server (`make dev` / Vite) — never kill or restart it. If browser automation is needed, use `playwright-cli` from `artifacts/claude/`: first check `command -v playwright-cli`, then fall back to `npx playwright-cli` if needed. Do **not** report browser automation unavailable merely because `require('playwright')` or `pnpm exec playwright` fails; those are different from the harness CLI. A passing build is not a rendering guarantee.
6. **Self-check** against `docs/UI-UX.md` §19 and the *rejected patterns* list before declaring done.

## Design craft

The system fixes palette, fonts, components, and motion. Within those bounds, a screen is still a series of design choices — make them deliberately:

- **Ground it in the subject.** Before designing, name the screen's concrete subject, its operator, and its single job — in this repo that means the business domain (preventivi, ordini, fornitori, target M&A…), who works it, and the decision or task it serves. Design with real content and the domain's own vocabulary; check existing UI labels before inventing terminology.
- **The opening is a thesis.** Lead with the most consequential data or action for the screen's job. A hero shell with big numbers, small labels, and a gradient accent is the template answer — this repo's UI review blocks it ("hero shells", "invented KPI filler"). Show summary numbers only when the operator actually acts on them.
- **Structure is information.** Numbering, eyebrows, dividers, and labels must encode something true about the content, never decorate it. Numbered markers (01/02/03) only when order genuinely carries meaning. Same for counts: never "N risultati" above a list the user already sees.
- **Personality lives in hierarchy.** With fonts and palette fixed, distinctiveness comes from layout, use of the type scale, spacing rhythm, and how the domain's data is presented. Precision *is* the aesthetic: a minimal direction executed well means exact spacing, deliberate weights, clean alignment. Elegance is executing the chosen vision well.
- **Motion is orchestrated, not scattered.** The system defines the entrance recipe (doc §8.2) and when not to animate (§8.3). One well-placed moment lands harder than effects everywhere; extra animation is what makes a design read as AI-generated.
- **Spend boldness in one place.** If a screen deserves a signature — an unusual but justified layout, a distinctive presentation of the domain's data — make it one thing and keep everything around it quiet and disciplined. Before shipping, look again and remove one accessory.
- **Critique the plan against the default.** Before implementing, ask: would this layout come out identical for any generic admin panel? If yes, it is a default, not a choice — revise the part that reads as template and say what changed. During the smoke test, take a screenshot and actually look at it; a picture is worth a thousand tokens.
- **CSS craft.** Keep selector specificity flat (CSS Modules help): section-level and element-level selectors that silently cancel each other's paddings/margins are a recurring failure mode. Build to the quality floor without announcing it: responsive down to mobile, visible keyboard focus, reduced motion respected (all specified in the doc).

## Copy

Words are design material, not decoration — they exist to make the screen easier to understand and use. UI copy is **Italian**, in the repo's dry B2B register (Stripe/Linear, not Medium):

- Write from the operator's side of the screen: name things by what people control and recognize, never by how the system is built (no raw transport/auth/system copy in the UI).
- Active voice, exact verbs: a control says precisely what it does ("Salva modifiche", not a generic submit). An action keeps the same name through the whole flow — the button "Pubblica" produces the toast "Pubblicato". Consistent vocabulary is how operators learn the product.
- Dry nouns, sentence case, no first person, no rhetorical questions, no filler. Designer prose (mental models, anthropomorphisms, explanatory metaphors) is not copy — filter it out before it reaches the interface.
- Errors give direction, not mood: state what went wrong and how to fix it; never apologize, never be vague.
- An empty screen is an invitation to act: follow doc §14.5 and include one primary action when the user can resolve the state.
- Each element does exactly one job: a label labels, an example demonstrates, nothing quietly does double duty.

## Known rejected patterns

Recurring findings this repo's reviews keep blocking. They complement (not replace) `portal-miniapp-ui-review`:

- Decorative counters ("N risultati", "N campi") when the user already sees the list.
- Decorative dataviz where actionable numbers belong (use €/giorno + €/mese, not percentage stackbars).
- Side-by-side comparison layouts for mutually exclusive choices.
- Forced master-detail when the two views need independent filters or exports.
- Workload presets (S/M/L) where operators specify exact resources.
- Internal cost/pipeline data (€/azienda, bande, survivor-rate) on user-facing surfaces — operator/dev surfaces only.
