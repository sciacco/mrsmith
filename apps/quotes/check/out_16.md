# Task reference

`check_16.md`

## Verification target

Kit-picker source loading.

## Appsmith factual source

- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Dettaglio/queries/get_kit_internal_names/get_kit_internal_names.txt`
- `apps/quotes/quotes-main.tar.gz` → `quotes-main/pages/Nuova Proposta/queries/list_kit/list_kit.txt`

## Migrated implementation evidence

- `apps/quotes/src/api/queries.ts` → `useKits`
- `apps/quotes/src/components/KitPickerModal.tsx`
- `apps/quotes/src/api/types.ts` → `Kit`

## Comparison

Both implementations load a kit list that excludes inactive or ecommerce kits and use it to drive kit selection. Appsmith exposes two concrete SQL variants; the migrated frontend uses one endpoint `/quotes/v1/kits` and expects kit id, internal name, prices, and category metadata.

## Outcome

`PARTIAL MATCH`

## Differences

- Appsmith detail picker only reads `id` and `internal_name`; the migrated picker also expects `nrc`, `mrc`, and `category_name`.
- The exact filter set behind `/quotes/v1/kits` is not visible in `apps/quotes`.

## Evidence

- Appsmith SQL examples:
  - `select id, internal_name from products.kit where is_active = true and ecommerce = false`
  - `select pc.name as category, k.internal_name, ... where k.is_active = true and (k.ecommerce = false )`
- Migrated hook: `api.get<Kit[]>('/quotes/v1/kits')`
- Migrated modal groups by `category_name` and displays `NRC` / `MRC`.

## Notes

- The data source purpose matches, but exact query equivalence remains partially opaque.
