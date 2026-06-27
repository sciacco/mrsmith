# Binocolo model evaluations

Manual evaluation notes for Binocolo Target M&A LLM scopes.

Date: 2026-06-27

## Summary

| Model | Scope | Verdict | Decision |
|---|---|---|---|
| `nvidia/nemotron-3-ultra-550b-a55b` | `ma_strategy_intent` | good on observed prompt | keep in shortlist |
| `openai/gpt-5.4` | `ma_strategy_intent` | good on clarified `MSP` + `saas` territory | temporary production default; keep evaluating |
| `openai/gpt-5.4-mini` | `ma_strategy_intent` | preserves `MSP` sector | shortlist; keep regression check on group-size turnover |
| `openai/gpt-5.4-nano` | `ma_strategy_intent` | misses `MSP` sector | reject for acronym intent until fixed |
| `mistralai/mistral-medium-3-5` | `ma_strategy_intent` | misses `MSP` sector | reject for acronym intent until fixed |
| `openai/gpt-5.4` | `ma_strategy_ateco_hierarchy` | consistent good `MSP` + `saas` mapping | temporary production default; keep evaluating |
| `openai/gpt-5.2` | `ma_strategy_ateco_hierarchy` | good `MSP` mapping, variable granularity | shortlist; validate granularity, missingCriteria policy and latency |
| `openai/gpt-5.4-mini` | `ma_strategy_ateco_hierarchy` | wrong/no-tool mapping for `MSP` | reject for ATECO hierarchy |
| `x-ai/grok-4.3` | `ma_strategy_ateco_hierarchy` | clean output, variable granularity | shortlist; validate redundancy/granularity |
| `google/gemini-3.1-flash-lite` | `ma_strategy_ateco_hierarchy` | usable but unstable | not default; use only with fallback/escalation |

## Current Production Decision

Use `openai/gpt-5.4` as the temporary production default for the evaluated Binocolo M&A LLM scopes while more test cases are collected.

Rationale:

- best current balance between quality and latency for `ma_strategy_ateco_hierarchy`;
- handles `MSP` + `saas` without broad `82` fallback, false non-sector `missingCriteria`, or tool loops;
- on the clarified mixed-territory prompt, preserves included provinces and excluded province correctly;
- significantly faster than `openai/gpt-5.2` on ATECO hierarchy while cleaner than the faster but unstable alternatives.

Known follow-up checks:

- validate SaaS adjacency policy for weak `62.10.00`;
- continue testing mixed region/province wording;
- keep fallback/escalation available for tool-loop or ambiguous-sector failures.

## Prompt Under Evaluation

```text
azienda che si occupa di servizi gestiti in umbria ed in provincia di ancona con almeno 3 dipendenti 2 o 3 soci , non facente parte di gruppo di dimensioni superiori ai 30 milioni
```

Related failing variant:

```text
MSP in umbria ed in provincia di ancona con almeno 3 dipendenti 2 o 3 soci , non facente parte di gruppo di dimensioni superiori ai 30 milioni
```

Additional mixed-sector variant:

```text
MSP in liguria esclusa imperia , cuneo e lucca, 3 soci max, circa 3M , saas
```

Clarified mixed-sector variant:

```text
MSP a cuneo e lucca e liguria esclusa imperia, 3 soci max, circa 3M , saas
```

## `openai/gpt-5.4` - `MSP` + `saas` Scenario

Observed result: succeeded on both `ma_strategy_intent` and `ma_strategy_ateco_hierarchy` across the mixed-sector variants.

### Intent

For the first wording:

```text
MSP in liguria esclusa imperia , cuneo e lucca, 3 soci max, circa 3M , saas
```

the model treated `imperia`, `cuneo` and `lucca` as excluded provinces. That is defensible because the source wording can be read as a single exclusion list.

For the clarified wording:

```text
MSP a cuneo e lucca e liguria esclusa imperia, 3 soci max, circa 3M , saas
```

the intent extraction matched the intended geography:

- `sectors.include`: `MSP` and `saas`, grounded in the exact phrases;
- `territory.includeRegions`: `liguria`;
- `territory.includeProvinces`: `cuneo`, `lucca`;
- `territory.excludeProvinces`: `imperia`;
- `turnover.around`: `3000000`, grounded in `circa 3M`;
- `maxShareholders.value`: `3`, grounded in `3 soci max`;
- `employees.min/max`: `null`, correctly absent;
- `constraints`: empty, correctly absent.

Territory note:

- the phrase `MSP in liguria esclusa imperia , cuneo e lucca` is easy to read as a single exclusion list;
- the clarified wording `MSP a cuneo e lucca e liguria esclusa imperia` resolved correctly, so this looks more like source ambiguity than a hard model defect.

Observed latency and tokens:

- about `2.3s-2.6s`;
- `1,063-1,067` total tokens;
- `686` prompt tokens and `377-381` completion tokens.

Classification:

```text
model: openai/gpt-5.4
scope: ma_strategy_intent
result: good_on_clarified_msp_saas_territory
strengths: preserves both MSP and saas, handles included and excluded provinces when wording is clear, keeps turnover around separate from constraints
weaknesses: territory and sector casing not normalized; ambiguous wording can be read as a single exclusion list
decision: shortlist for intent; validate on more mixed-sector prompts
```

### ATECO Hierarchy

The ATECO resolver consistently selected the same codes for the mixed-sector variants.

Tool calls:

- `62` - programming, IT consultancy and related activities;
- `63` - IT infrastructure, data processing, hosting and information services.

The first run also inspected `61` (telecommunications). The clarified run avoided that extra call and used only `62` and `63`.

Selected codes:

| Code | Fit | Assessment |
|---|---|---|
| `62.20.20` | strong | strong MSP mapping to management of IT structures |
| `63.10.10` | strong | strong SaaS/hosting/infrastructure mapping |
| `62.10.00` | weak | plausible adjacent software-development component for SaaS, but not sufficient alone |

Positive signals:

- no broad `82` fallback;
- no false non-sector `missingCriteria`;
- no tool loop;
- selected codes are grounded in visited taxonomy labels;
- strong leaf-level choices for both MSP and SaaS.

Residual risk:

- one earlier irrelevant `61` tool call;
- `62.10.00` for SaaS development is plausible but product policy should decide whether to include software development as weak adjacency whenever `saas` is present.

Observed latency and tokens:

- about `3.5s-3.7s`;
- `5,126-5,924` total tokens;
- `4,948-5,747` prompt tokens and `177-178` completion tokens.

Classification:

```text
model: openai/gpt-5.4
scope: ma_strategy_ateco_hierarchy
result: good_on_msp_saas_mapping
strengths: stable selected codes, no missingCriteria noise, fast compared with gpt-5.2, latest run uses only relevant 62/63 tool calls
weaknesses: one earlier extra 61 inspection, weak SaaS-development adjacency needs policy validation
decision: strong shortlist for ATECO hierarchy; validate SaaS adjacency policy before default
```

## Intent Extraction On `MSP`

Additional `ma_strategy_intent` runs tested the acronym wording:

```text
MSP in umbria ed in provincia di ancona con almeno 3 dipendenti 2 o 3 soci , non facente parte di gruppo di dimensioni superiori ai 30 milioni
```

The key discriminator is whether the model preserves `MSP` as a sector include. Downstream ATECO resolution has no safe sector constraint if the intent step drops the acronym.

### `openai/gpt-5.4-mini` - `ma_strategy_intent`

Observed result: succeeded in the observed `MSP` runs, with the sector preserved.

Correct or acceptable fields:

- `sectors.include`: `MSP`, grounded in `MSP`;
- `territory.includeRegions`: `Umbria`/`umbria`, grounded in `umbria` or `in umbria`;
- `territory.includeProvinces`: `Ancona`/`ancona`, grounded in `provincia di ancona`;
- `employees.min`: `3`, grounded in `almeno 3 dipendenti`;
- `maxShareholders.value`: `3`, grounded in `2 o 3 soci`;
- `constraints`: `group_size`, `disposition=deep`, grounded in `non facente parte di gruppo di dimensioni superiori ai 30 milioni`;
- `thesis`: empty string.

Residual risk:

- one earlier run populated `turnover.max=30000000` from the group-size phrase. The latest observed runs kept `turnover.max=null`, which is cleaner because group-size is already represented as a deep constraint. Keep this as a regression check.

Observed latency and tokens:

- about `2.3s-2.7s`;
- `1,076-1,095` total tokens;
- `697` prompt tokens and `379-398` completion tokens.

Classification:

```text
model: openai/gpt-5.4-mini
scope: ma_strategy_intent
result: good_on_msp_acronym
strengths: preserves MSP, extracts territory and non-sector constraints correctly
weaknesses: one earlier run duplicated group-size into turnover, minor casing variation on territory
decision: shortlist for intent; keep group-size turnover behavior under regression
```

### Failed `MSP` Intent Extractors

The following models completed successfully at transport/schema level, but returned an empty sector include list:

```json
"sectors": {"exclude": [], "include": []}
```

This is a material miss. `MSP` is the sector signal in this prompt and should be preserved as a sector include, even if the model does not expand it to "managed service provider".

### `openai/gpt-5.4-nano` - `ma_strategy_intent`

Observed result: succeeded, but sector extraction failed.

Correct or acceptable fields:

- `territory.includeRegions`: `Umbria`, grounded in `in umbria`;
- `employees.min`: `3`, grounded in `almeno 3 dipendenti`;
- `maxShareholders.value`: `3`, grounded in `2 o 3 soci`;
- `constraints`: `group_size`, `disposition=deep`, grounded in `non facente parte di gruppo di dimensioni superiori ai 30 milioni`;
- `thesis`: empty string.

Defects:

- `sectors.include` is empty even though `MSP` is the main sector criterion;
- `territory.includeProvinces.value` was returned as `provincia di ancona` instead of normalized `Ancona`.

Observed latency and tokens:

- about `3.4s`;
- `1,057` total tokens;
- `697` prompt tokens and `360` completion tokens.

Classification:

```text
model: openai/gpt-5.4-nano
scope: ma_strategy_intent
result: bad_on_msp_acronym
strengths: extracts non-sector constraints correctly
weaknesses: drops the only sector criterion, weak province normalization
decision: reject for acronym-heavy intent prompts until prompt/rules are fixed
```

### `mistralai/mistral-medium-3-5` - `ma_strategy_intent`

Observed result: succeeded, but sector extraction failed.

Correct or acceptable fields:

- `territory.includeRegions`: `umbria`, grounded in `umbria`;
- `territory.includeProvinces`: `ancona`, grounded in `provincia di ancona`;
- `employees.min`: `3`, grounded in `almeno 3 dipendenti`;
- `maxShareholders.value`: `3`, grounded in `2 o 3 soci`;
- `constraints`: `group_size`, `disposition=deep`, grounded in `non facente parte di gruppo di dimensioni superiori ai 30 milioni`;
- `thesis`: empty string.

Defects:

- `sectors.include` is empty even though `MSP` is the main sector criterion;
- territory casing is not normalized (`umbria`, `ancona`), though the values are still usable.

Observed latency and tokens:

- about `2.8s`;
- `1,005` total tokens;
- `727` prompt tokens and `278` completion tokens.

Classification:

```text
model: mistralai/mistral-medium-3-5
scope: ma_strategy_intent
result: bad_on_msp_acronym
strengths: extracts territory and non-sector constraints correctly
weaknesses: drops the only sector criterion, weak casing normalization
decision: reject for acronym-heavy intent prompts until prompt/rules are fixed
```

### Intent Takeaway

The `ma_strategy_intent` prompt needs an explicit rule for business acronyms:

```text
If the request starts with or contains an acronym that denotes a business type or sector (for example MSP, MSSP, VAR, SOC, NOC), preserve that acronym in sectors.include with the exact sourceText. Do not drop it merely because it is an acronym.
```

Until that prompt rule is tested, `openai/gpt-5.4-nano` and `mistralai/mistral-medium-3-5` should not be used as default intent extractors for acronym-heavy M&A prompts. `openai/gpt-5.4-mini` already preserves `MSP`, so it is the better OpenAI candidate for the intent scope among the observed runs.

## `nvidia/nemotron-3-ultra-550b-a55b` - `ma_strategy_intent`

Observed result: succeeded.

The model extracted the structured intent correctly and consistently across the observed runs:

- `territory.includeRegions`: `Umbria`, grounded in `in umbria`;
- `territory.includeProvinces`: `Ancona`, grounded in `in provincia di ancona`;
- `sectors.include`: `servizi gestiti`;
- `employees.min`: `3`, grounded in `almeno 3 dipendenti`;
- `maxShareholders.value`: `3`, grounded in `2 o 3 soci`;
- `constraints`: `group_size`, `disposition=deep`, grounded in `non facente parte di gruppo di dimensioni superiori ai 30 milioni`;
- `thesis`: empty string, correctly not copied from the full prompt.

Observed latency:

- about `1.8s-2.2s`;
- about `1,071` total tokens per call.

Verdict: good for `ma_strategy_intent` on this prompt. Keep in the shortlist for the intent extraction scope.

## `openai/gpt-5.2` - `ma_strategy_ateco_hierarchy`

Observed result: succeeded on the `MSP` acronym prompt and used the hierarchy resolver in both observed runs.

The model consistently inspected the relevant IT divisions:

- `62` - programming, IT consultancy and related activities;
- `63` - IT infrastructure, data processing, hosting and information services.

One run also inspected:

- `61` - telecommunications.

The `61` inspection is extra, but it did not contaminate the final selected codes. The later run avoided it and inspected only `62` and `63`.

Observed selected-code shapes:

| Run shape | Selected codes | Assessment |
|---|---|---|
| intermediate coverage + adjacent consulting | `62.20.2` strong, `63.10.1` strong, `62.20.1` weak | broad but defensible MSP coverage; includes consulting as weak adjacent |
| leaf-focused coverage | `62.20.20` strong, `63.10.10` weak | cleaner and more specific; maps managed IT structures directly and treats hosting as optional |

One run returned `MSP` in `missingCriteria` because the acronym can cover different managed-service perimeters:

```json
{"text": "MSP", "reason": "Il termine puo' riferirsi a perimetri diversi ..."}
```

The later run returned an empty `missingCriteria`, which is cleaner because it already mapped `MSP` to the relevant visited taxonomy labels. The earlier ambiguity warning is defensible, but it creates a product-policy decision: if selected codes already cover the best interpretation, downstream code must decide whether this should be surfaced as a missing criterion or treated as a low-confidence note.

Observed latency and tokens:

- about `14.3s-15.2s`;
- `5,410-6,319` total tokens;
- `4,957-5,766` prompt tokens and `453-553` completion tokens.

Classification:

```text
model: openai/gpt-5.2
scope: ma_strategy_ateco_hierarchy
result: good_on_msp_mapping_variable_granularity
strengths: relevant tool use, correct IT-focused selections, avoids broad 82 fallback, latest run has clean missingCriteria
weaknesses: slower than other candidates, one extra 61 inspection, variable parent/leaf granularity and optional hosting/consulting coverage
decision: shortlist for ATECO hierarchy; validate granularity, missingCriteria handling and latency before default use
```

## `openai/gpt-5.4-mini` - `ma_strategy_ateco_hierarchy`

Observed result: succeeded at API/schema level, but failed the business mapping.

Input sector was preserved by the previous intent step:

```json
"includeSectors": [{"text": "MSP", "sourceText": "MSP"}]
```

The model did not call `list_ateco_children`. It selected only the broad division:

| Code | Fit | Assessment |
|---|---|---|
| `82` | weak | wrong direction for MSP; generic business support is not a safe proxy for managed IT services |

It also returned `MSP` in `missingCriteria`:

```json
{"text": "MSP", "reason": "acronimo non univoco nel taxonomy ATECO fornito; non e' possibile mapparlo con sicurezza a una specifica attivita' economica"}
```

This is internally inconsistent: if `MSP` is too ambiguous to map, selecting `82` as a fallback broad division still broadens the strategy. If it is interpreted as Managed Service Provider in the M&A/IT context, the model should inspect IT divisions such as `62` and `63` and select from visited descendants.

Defects:

- no tool call despite an available hierarchy resolver;
- selected `82`, an irrelevant broad parent for MSP/managed IT services;
- did not inspect `62` or `63`;
- returned both a weak selected code and a missing criterion for the same sector;
- violates the "do not broaden the strategy to hide uncertainty" rule.

Observed latency and tokens:

- about `2.2s`;
- `3,796` total tokens;
- `3,682` prompt tokens and `114` completion tokens.

Classification:

```text
model: openai/gpt-5.4-mini
scope: ma_strategy_ateco_hierarchy
result: bad_on_msp_mapping
strengths: fast, valid JSON shape
weaknesses: no hierarchy tool use, wrong broad 82 selection, inconsistent missingCriteria
decision: reject for ATECO hierarchy on MSP prompts
```

## `x-ai/grok-4.3` - `ma_strategy_ateco_hierarchy`

Observed result: succeeded across the pasted runs.

The model called `list_ateco_children` only for the two relevant IT divisions:

- `62` - programming, IT consultancy and related activities;
- `63` - IT infrastructure, data processing, hosting and information services.

This is cleaner than the Gemini run on the same prompt, which also explored `82`.

Observed selections varied by run:

| Run shape | Selected codes | Assessment |
|---|---|---|
| narrow leaf | `62.20.20` | clean, focused on managed IT structures |
| intermediate + leaf | `62.20.2`, `62.20.20` | semantically correct but redundant |
| parent + intermediate + leaf + weak adjacent | `62.20`, `62.20.2`, `62.20.20`, `63.10` weak | broadest result; useful coverage, but contains overlapping nodes |

Positive signals:

- no false non-sector `missingCriteria`;
- no observed tool loop;
- no irrelevant division exploration;
- selected codes are all grounded in visited ATECO labels;
- understands managed IT services / MSP wording in the rationale.

Residual risks:

- granularity is not stable: the model alternates between parent, intermediate and leaf nodes;
- some runs select overlapping nodes such as `62.20`, `62.20.2` and `62.20.20` together;
- one run selects only `62.20.20`, which may under-cover hosting/infrastructure-style managed services if product wants broader MSP coverage.

Observed latency and tokens:

- about `5.2s-6.2s`;
- about `5,003-5,036` total tokens per call;
- about `4,764` prompt tokens and `239-272` completion tokens.

### Verdict

`x-ai/grok-4.3` is a strong candidate for `ma_strategy_ateco_hierarchy` on this prompt. It is slower than Gemini but cleaner: no false missing criteria, no noisy `82` exploration, and no observed loop.

It should stay in the shortlist, but not be promoted blindly until the granularity policy is settled. The key open question is whether overlapping selections should be accepted, deduplicated by backend post-processing, or considered a model defect.

Classification:

```text
model: x-ai/grok-4.3
scope: ma_strategy_ateco_hierarchy
result: good_but_granularity_variable
strengths: clean missingCriteria, relevant tool calls, no observed loop
weaknesses: overlapping parent/intermediate/leaf selections, variable coverage of 63.10
decision: shortlist; validate with more prompts before default
```

## `google/gemini-3.1-flash-lite` - `ma_strategy_ateco_hierarchy`

Observed result: mixed.

On the explicit wording `servizi gestiti`, the model completed multiple runs successfully. It called `list_ateco_children` for:

- `62` - programming, IT consultancy and related activities;
- `63` - IT infrastructure, data processing, hosting and information services;
- `82` - office support and other business support services.

The `82` exploration is extra/noisy, but it did not contaminate the final selected codes in the successful runs.

Successful selections were stable and reasonable:

| Code | Fit | Reason |
|---|---|---|
| `62.20.20` | strong | managed operation of IT structures maps well to managed IT services |
| `63.10.10` | strong | IT infrastructure and hosting are core managed-service components |

Observed latency and tokens:

- about `2.4s-3.9s`;
- about `6,223-6,354` total tokens per call;
- prompt/input dominates cost (`~6,083` prompt tokens), completion is small (`140-271` tokens).

### Defects

1. False non-sector `missingCriteria`

Some successful runs emitted missing criteria for constraints handled outside ATECO:

- `almeno 3 dipendenti`;
- `2 o 3 soci`;
- `non facente parte di gruppo di dimensioni superiori ai 30 milioni`.

This violates the `ma_strategy_ateco_hierarchy` prompt. The current backend guard in `constrainMAAtecoRerank` should filter these out before they reach the final strategy, but the model output is still lower quality than a clean pass.

2. Tool loop on acronym wording

The model failed on the acronym variant `MSP ...` with:

```text
invalid ma strategy: ateco hierarchy tool loop
```

This is a hard failure from the backend guard after the model keeps requesting `list_ateco_children` instead of selecting from already returned descendants.

### Verdict

`google/gemini-3.1-flash-lite` is fast and can produce good ATECO selections for explicit Italian sector wording such as `servizi gestiti`, but it is not robust enough to be the default for `ma_strategy_ateco_hierarchy`.

Classification:

```text
model: google/gemini-3.1-flash-lite
scope: ma_strategy_ateco_hierarchy
result: usable_but_unstable
strengths: fast, good selections on "servizi gestiti"
weaknesses: false non-sector missingCriteria, tool loop on "MSP"
decision: not default; candidate only with fallback/escalation
```

Recommended fallback policy:

- keep a stronger default model for `ma_strategy_ateco_hierarchy`;
- allow `google/gemini-3.1-flash-lite` only behind automatic escalation;
- escalate on `ateco hierarchy tool loop`;
- escalate when the sector contains acronyms such as `MSP`, `MSSP`, `VAR`, `SOC`, `NOC`, or similar business shorthand;
- do not increase tool rounds as the primary fix, because it hides the model-compliance issue and worsens latency/cost.
