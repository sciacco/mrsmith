# Panoramica Cliente

Knowledge entries specific to `apps/panoramica-cliente`.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### Cloudstack IaaS Charge Categories Are Fixed Backend-Side

- Context: IaaS Pay Per Use consumption views and any future billing/exploration surface over Grappa `cdl_charges`.
- Discovery: `cdl_charges.usage_type` is a flat code list. For human-facing consumption analysis it is grouped into fixed macro-categories, and `usage_type = 9999` (Credit) is a bookkeeping line, not real consumption, so it is excluded from all consumption totals and from the category composition.
- Practical rule: group usage_type into **VM** (2), **Storage** (6,7,8,9), **Licenze Windows** (9998), **Altro** (1,3,26,27,NULL,unknown), and always exclude 9999. Compute the period total as the sum of the four returned categories so the KPI total and the pie stay consistent. Apply the same grouping in any new IaaS consumption surface instead of re-deriving it.
- Evidence: `backend/internal/panoramica/handler_iaas.go` (`categoryFromUsageType`, `handleChargesByCategory`); `apps/panoramica-cliente/SPEC.md` IaaS entity/view sections.
- Used by: `apps/panoramica-cliente` IaaS Pay Per Use.
- Open questions: none.
