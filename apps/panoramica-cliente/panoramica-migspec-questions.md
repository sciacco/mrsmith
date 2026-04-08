# Consolidated Questions — Panoramica Cliente Migration Spec

## All questions resolved.

| # | Decision |
|---|----------|
| 1 | **As-is.** Each Ordini page keeps its own dismissal filter. Customer selection is always required on both pages (no "Tutti i clienti" option). |
| 2 | **Null = no filter.** Backend accepts `null` as "no date limit" (replaces 2000-month hack). |
| 3 | **As-is.** `get_reverse_order_history_path()` stays in query. |
| 4 | **Drop.** Checkbox was Appsmith workaround. |
| 5 | **Hardcoded as-is.** Line status list stays static. |
| 6 | **As-is.** Use Mistra `loader.grappa_*` copies. |
| 7 | **Separate pages.** Licenze Windows = riepilogo complessivo, not per-account. |
| 8 | **As-is.** Exclusion codes not centralized. |
| 9 | **Exclude `KlajdiandCo`** via WHERE in tenant query. |
| 10 | **As-is.** Latest PBX snapshot only. |
| 11 | **Independent.** No tenant-customer relationship. |
| 12 | **Groups:** Ordini, Fatture, Servizi (Accessi + IaaS PPU + Timoo + Licenze Windows). |
| 13 | **Yes.** CSV export enabled on all tables. |
| 14 | **Standardize.** Accessi uses labeled "Cerca" button like other pages. |
| 15 | **As-is.** Usage type labels stay English technical names (utRunningVM, etc.). |
| 16 | **Hidden.** `cloudstack_domain` UUID not shown in table (as-is). |
| 17 | **Auto-load.** Timoo PBX stats load on tenant selection (no button). |
| 18 | **Typed array.** Backend returns `[{type, label, amount}]` for charge breakdown. |
| 19 | **As-is.** Backend computes grouping in SQL (NULL on non-first rows). |
| 20 | **Out of scope.** Loader refresh frequency not relevant to migration. |
| 21 | **As-is.** Each endpoint uses its original ID type. |
| 22 | **`app_panoramica_access`.** |
| 23 | **Follow convention** of existing apps. |

All 23 questions resolved. Ready for Phase E (Specification Assembly).
