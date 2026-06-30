# UC2 Sector-Analyst Model Evaluation & Grounding Ablation

Evaluation of the LLM analyst used in **UC2 web sector classification** (scope
`candidate_match_analyst`): which model to run, whether to serve DeepSeek via Fireworks
or directly, and whether the embedded-KB **concept-match grounding** actually helps the
analyst.

- **Date:** 2026-06-30
- **Branch:** `poc/aenad`
- **Harness:** `POST /binocolo/v1/test/sector-eval-models` (offline replay over labeled sessions)
- **Test set:** 2 fully-labeled sessions, **54 companies** total
  - `391e7c67-f611-41f7-899f-0a396e4b8d2f` — "Target IT Nord-Est e Lombardia 3-6M" (36 cases)
  - `84781682-98b6-442d-877f-a1d9b72a8b9e` — "MSP / cybersecurity / virtualizzazione, Liguria" (18 cases)
- Related: [`KB-ENRICHMENT-BRIEF.md`](KB-ENRICHMENT-BRIEF.md), [`MODELS-EVALS.md`](MODELS-EVALS.md) (separate scopes), [`business-maps/TAXONOMY-SPINE.md`](business-maps/TAXONOMY-SPINE.md)

---

## TL;DR

1. **Model viability** — of the backups tried, `gemma-4-31b` is **unusable** (truncates/degenerates its JSON ~50-70% of calls). `gpt-oss-120b`, `mimo-v2.5`, `mimo-v2.5-pro`, and DeepSeek (both hosts) all run clean (0 failures).
2. **Quality is a statistical tie across the serious contenders.** DeepSeek-via-Fireworks (current prod), DeepSeek-direct, and gpt-oss-120b are indistinguishable on accuracy/scarta-recall over 10× runs (per-run noise ≈ ±0.03 dwarfs the gaps). Single-run "winners" were noise.
3. **The differentiators are cost and latency, not quality.** `gpt-oss-120b` is **~15× faster** and **~25% cheaper** than DeepSeek at equal quality. DeepSeek-direct is **~24% faster and ~12% cheaper** than the same model via Fireworks.
4. **The embedded grounding (per-company concept matches) does not earn its keep as analyst input — it hurts.** Ablating it is neutral-to-positive for DeepSeek and **strongly positive for gpt-oss**. Its only consistent effect across both models is to **bias toward keeping** (more false-keeps *and* more keep-leaks). Confirmed under **production-faithful input (with raw snippets, §7.1)**: grounding is net-harmful to both models — decisively for gpt-oss (accuracy −0.098, scartaRecall −0.150, t≈−12). The single best configuration in the whole study is **`gpt-oss-120b` with grounding OFF** (accuracy 0.822, scartaRecall 0.895, keepLeak 0, tightest variance).
5. **Recommendations:** (a) prefer `gpt-oss-120b` as analyst (speed/cost at tied quality), with DeepSeek-direct as the same-family fallback; (b) **drop the concept-match grounding from the analyst payload** — the gating snippet-faithful re-test confirmed it is net-harmful (embed+rerank would remain for UC1 retrieval only); (c) **[implemented, staged for review]** retire the deterministic terminal-reject gate — replaced by a threshold-free **structural** rule (terminal-reject only when the top concept is a distractor *and* nothing in-perimeter matched), so the gate can no longer leak a true keep (`sectorVerdict`, migration 069).

> All numbers below are **analyst-on-all** (the analyst decides every company), which is the
> "solo-LLM" path — not the production hybrid (deterministic gate + LLM only on ambiguous).
> Read them as a model/grounding comparison, not as the live pipeline's accuracy.

---

## 1. Background

### 1.1 Where the analyst sits in UC2

```
domain resolution (Brave) → verifyDomainByScrape → gatherNeutralEvidence
   → representCompany (distil neutral description)
   → classifyCompanyConcepts  ── embed recall + Qwen3 rerank over the curated KB ──┐
   → sectorVerdict (deterministic: confirm/reject/weak/ambiguous/no_signal)        │ "the embedded data"
   → analyzeSectorAmbiguity (LLM analyst) ◄── fed the concept matches as grounding ─┘
   → sectorFinalDecision → bucket {keep | forse | scarta}
```

The **embedded data** = the curated KB concept vectors (targets + distractors) plus the
reranker. It is used two ways:

- **As the deterministic gate** (`sectorVerdict`): embed+rerank alone decide a verdict.
  Prior replay work (`ma_sector_replay.go`) found this tops out ≈0.63 accuracy as a
  standalone classifier (below the LLM's ≈0.75) and the terminal-reject branch leaks true
  keeps. *Out of scope for this doc; it is the pending "terminal-0.04 reversal".*
- **As grounding for the analyst**: the matched concepts (name, kind=target/distractor,
  in-perimeter flag, rerank prob, sibling discriminator) are handed to the LLM as
  structured evidence. **This is what §5 ablates.**

### 1.2 The eval harness (labels & metrics)

Inverted labeling: web validation is run over a whole session, then the operator confirms
or reclassifies each company into **keep / forse / scarta**; that human label is ground
truth (`ma_sector_eval_label`, migration 066). Metrics used throughout:

| Metric | Definition | Why it matters |
|---|---|---|
| **accuracy** | correct bucket / evaluable | overall |
| **keepLeak** | `label=keep` predicted `scarta` | **the recall-safety violation** — a true target terminally lost |
| **keepRecall** | keep→keep / all keep labels | are we retaining targets? |
| **scartaRecall** | scarta→scarta / all scarta labels | are we dropping off-targets? |
| **false-keeps** | `label=scarta` predicted `keep` | off-targets wrongly promoted into the shortlist |

> **Test-set shape caveat (important):** the 54 labeled companies are **scarta-heavy**
> (~44 scarta, ~7 keep, ~3 forse). So **scarta-side metrics are statistically solid; all
> keep-side metrics (keepRecall, keepLeak) are noisy** — with only 7 keeps, one company is
> ~0.14 of keepRecall. Treat keep-side deltas as weak signal.

---

## 2. The comparison endpoint

`POST /binocolo/v1/test/sector-eval-models` — replays the analyst over labeled sessions
with N models in parallel, **same prompt, only the model varies**, and scores each against
ground truth. Read-only (no persistence beyond the per-call LLM audit).

**Request**
```json
{
  "sessionIds": ["<uuid>", "..."],
  "modelIds":   ["<uuid>", "..."],     // resolved by id regardless of scope; [] → default analyst
  "maxTokens":  16000,                  // optional: force max_tokens (reasoning models)
  "perCallTimeoutMs": 28000,            // optional: cap each call; slow call → counted failure
  "dropConceptMatches": false,          // optional: ablation — omit the embed+rerank grounding
  "includeSnippets": false              // optional: re-gather raw web snippets (cached per domain)
}
```

**Response (per model):** `accuracy`, `keepLeak`, `keepRecall`, `scartaRecall`, full
`confusion[label][pred]`, `failures`, `sampleError` (first failure — *why* a model breaks),
`tokens`, `avgLatencyMs`; plus a per-company `items[]` disagreement table with each model's
bucket and an `agree` flag.

**Engineering notes / gotchas**
- **Input reconstruction:** the analyst input is rebuilt from the persisted summary —
  distilled description + concept matches (with the `Contrast` discriminator re-attached
  from the KB) + perimeter (`deriveStrategyConcepts`).
  **Fidelity caveat:** raw web snippets are *not* persisted (they live only in the
  write-only trace), so the replayed analyst sees description + concepts + perimeter but
  **not snippets** — slightly thinner than production. Fair for *relative* comparison (every
  model/condition gets the identical input); not an absolute production number.
- **60s WriteTimeout (server, `main.go`):** the endpoint is synchronous, so a request must
  finish in <60s or the whole result is lost (HTTP 000). A 2-model × 2-session run (138
  calls) overran (175s) and lost everything. **Run per-session, and split per-model for slow
  models.** Concurrency was raised `12 → 20` (`maSectorCompareConcurrency`); `perCallTimeoutMs`
  was added so a high-tail-latency reasoning model can't sink a request.
- **Param passthrough:** the model row's `params` jsonb is forwarded verbatim to the
  top-level request body, so provider-specific controls (e.g. DeepSeek `{"thinking":{"type":"disabled"}}`,
  `reasoning_effort`) are **config-only — no code change**.

---

## 3. Methodology

- **Buckets** keep/forse/scarta as above; the analyst verdict→bucket mapping is
  `analystToBucket` (confirm+strong/match → keep; reject/no_match → scarta; else forse).
- **Single-run screening** (§4) was used to decide *viability* and rough ranking.
- **10× runs** (§5–§6) were used wherever a decision depended on a quality difference,
  because the per-run accuracy noise is ≈ ±0.03. Each "rep" is one full pass over both
  sessions (54 companies); reps differ only by LLM nondeterminism.
- **Pairing:** when two models/conditions ran in the *same request* per rep, they saw the
  identical companies under identical conditions → paired comparison (per-rep deltas).
- **Drivers** (in `artifacts/claude/`, gitignored): `variance_driver.sh` (§5),
  `ablation_driver.sh` (§6), with `*_agg.mjs` aggregators computing mean ± std and paired
  deltas with a t-statistic.

---

## 4. Model arena (viability screen, single runs, 54 companies)

| Model | id | provider | acc | scartaRecall | keepLeak | false-keeps | lat/call | tokens | failures | verdict |
|---|---|---|---|---|---|---|---|---|---|---|
| gemma-4-31b | `c0cdf3d0`* | Cerebras | — | — | — | — | — | — | **~50-70%** | ❌ **unusable** — truncates JSON at 5000 tok, degenerates into repetitive garbage at 16000 |
| mimo-v2.5-pro | `6c0045f8` | Xiaomi | 0.648 | 0.705 | 1 | 0 | 10.3s | 97k | 0 | ⚠️ viable but weakest; "forse"-biased, high tail-latency |
| mimo-v2.5 | `c0cdf3d0`* | Xiaomi | 0.704 | 0.727 | 1 | **0** | 6.9s | 97k | 0 | ✅ viable, cautious (0 false-keeps) |
| gpt-oss-120b | `136233a7` | Cerebras | 0.759 | 0.795 | 1 | 4 | **0.7s** | **93k** | 0 | ✅ viable, fast/decisive |
| DeepSeek V4 Flash (Fireworks) | `282080c5` | Fireworks | 0.74–0.78 | 0.75–0.80 | 1 | 4 | 14s | 140k | 0 | current production analyst |
| DeepSeek V4 Flash (direct) | `97e9dad0` | DeepSeek | 0.796 | 0.841 | 1 | 2 | 10.5s | 126k | 0 | ✅ same model, cheaper/faster than via Fireworks |

\* `c0cdf3d0` was repointed out-of-band during testing (gemma-4-31b → mimo-v2.5); the LLM
registry binding changes in the DB independently of any seed. Always confirm the live binding.

**gemma-4-31b failure detail** (captured via `sampleError`): at `max_tokens=5000` it emits
clean JSON that cuts off mid-string (`"businessFit": "Fornitore di software gestionale`⟨cut⟩);
at 16000 it loops into garbage (`{", " : {", " : {"…`, 233k tokens). A model that cannot
reliably emit the structured verdict is disqualified regardless of accuracy.

Single-run numbers were enough to (a) drop gemma, (b) rank mimo-pro last, and (c) flag
DeepSeek-direct and gpt-oss as the serious contenders — but the apparent quality gaps among
the contenders did **not** survive variance analysis (§5–§6).

---

## 5. DeepSeek: direct vs via Fireworks (10× paired)

Same model (`deepseek-v4-flash`), two hosts. Driver: `variance_driver.sh` (10 reps).

| Metric (mean ± std, n=10) | via Fireworks | direct | verdict |
|---|---|---|---|
| accuracy | 0.754 ± 0.030 | 0.762 ± 0.024 | **tie** (Δ +0.008, t≈0.8) |
| scartaRecall | 0.782 ± 0.029 | 0.801 ± 0.018 | direct edge (Δ +0.019, 8/10) |
| keepRecall | 0.771 ± 0.120 | 0.757 ± 0.118 | tie (noisy — 7 keeps) |
| keepLeak | 1.10 ± 0.32 | 1.30 ± 0.48 | tie |
| false-keeps | 4.0 ± 1.5 | 2.7 ± 0.8 | direct fewer + steadier |
| **latency/call** | 14.0 s ± 0.8 | **10.6 s ± 0.9** | **direct ~24% faster** (non-overlapping) |
| **tokens** | 140k ± 2.4k | **124k ± 1.8k** | **direct ~12% cheaper** (non-overlapping) |

**Finding:** accuracy is a statistical tie — the single-run 0.944-vs-0.778 Liguria gap was a
lucky draw (Fireworks alone ranged 0.685–0.796 across reps). The **durable** wins for direct
are cost and latency (ranges don't overlap), plus a small, consistent scarta-recall edge and
fewer/steadier false-keeps. Serving the same model directly is a cost/latency upgrade at equal
quality — lowest-risk change (same model already trusted in prod).

DeepSeek-direct config: provider DeepSeek (`api.deepseek.com`), `model=deepseek-v4-flash`,
params `{"thinking":{"type":"disabled"}}`. Thinking defaults to *enabled*; with it on, the CoT
returns in a separate `reasoning_content` field (so our `content` parse stays clean) but
reasoning consumes the `max_tokens` budget — disabling it gives a fast, clean structured pass.

---

## 6. DeepSeek-direct vs gpt-oss-120b (10× paired, production/grounded config)

Both models in the same request per rep (perfectly paired). Source: grounded arm of the
ablation driver (§7).

| Metric (mean ± std, n=10) | DeepSeek-direct | gpt-oss-120b | verdict |
|---|---|---|---|
| accuracy | 0.766 ± 0.033 | 0.750 ± 0.024 | **tie** (Δ +0.016, t≈1.1) |
| scartaRecall | 0.808 ± 0.035 | 0.795 ± 0.024 | **tie** (Δ +0.013, t≈0.9) |
| keepRecall | 0.757 ± 0.118 | 0.757 ± 0.069 | identical |
| false-keeps | 3.0 ± 0.9 | 3.2 ± 0.6 | tie |
| keepLeak | 1.3 ± 0.5 | 1.0 ± 0.0 | tie |
| **latency/call** | 10.4 s | **0.68 s** | **gpt-oss ~15×** |
| **tokens** | 123k | **93k** | **gpt-oss −25%** |

**Finding:** dead heat on every quality metric; gpt-oss-120b is ~15× faster and ~25% cheaper.
When quality ties, gpt-oss wins on operational cost. (gpt-oss is Cerebras — also a different
provider from the current Fireworks path, so it doubles as provider-outage resilience.)

---

## 7. Does the embedded grounding help the analyst? (ablation, 10× paired)

`dropConceptMatches:true` omits the per-company embed+rerank concept matches from the analyst
payload, holding everything else constant (description + perimeter; snippets absent in both
arms). Driver: `ablation_driver.sh`. Effect reported as **grounded − ablated** (positive =
grounding helps), paired over 10 reps.

### Per-condition means (n=10)

| | DeepSeek-direct grounded | DeepSeek-direct ablated | gpt-oss grounded | **gpt-oss ablated** |
|---|---|---|---|---|
| accuracy | 0.766 ± 0.033 | 0.743 ± 0.033 | 0.750 ± 0.024 | **0.776 ± 0.024** |
| scartaRecall | 0.808 ± 0.035 | 0.772 ± 0.026 | 0.795 ± 0.024 | **0.834 ± 0.022** |
| keepRecall | 0.757 ± 0.118 | 0.755 ± 0.134 | 0.757 ± 0.069 | 0.629 ± 0.120 |
| false-keeps | 3.0 ± 0.9 | 1.3 ± 1.1 | 3.2 ± 0.6 | **1.8 ± 0.4** |
| keepLeak | 1.3 | **0.0** | 1.0 | **0.0** |

### Grounding effect (grounded − ablated, paired)

| metric | DeepSeek-direct | gpt-oss-120b |
|---|---|---|
| accuracy Δ | +0.023 (t≈1.5, ns) | **−0.026 (t≈−2.4 — grounding HURTS)** |
| scartaRecall Δ | +0.036 (t≈3.0, helps) | **−0.039 (t≈−3.6 — HURTS)** |
| keepRecall Δ | +0.002 (none) | +0.129 (t≈2.9 — helps) |
| false-keeps Δ | +1.7 (t≈3.8 — grounding **worse**) | +1.4 (t≈5.3 — grounding **worse**) |
| keepLeak | 1.3 → 0 when ablated | 1.0 → 0 when ablated |

**Findings:**

- **No clear benefit, and model-dependent direction.** Grounding modestly helps DeepSeek's
  scarta-recall (+0.036) but is **net-harmful to gpt-oss** on both accuracy (−0.026) and
  scarta-recall (−0.039), all significant.
- **The one consistent cross-model effect is a bias toward keeping.** Grounding *increases
  false-keeps* in both models (t≈3.8 and t≈5.3) and *increases keep-leaks* (both go 1.0–1.3 →
  0 when ablated). Hypothesis: a weak in-perimeter concept match reads to the analyst as
  positive evidence, pulling borderline off-targets into `keep` and occasionally mis-sorting
  true keeps.
- **The single best configuration in the whole study is `gpt-oss-120b` with grounding OFF:**
  accuracy 0.776, scarta-recall 0.834, **keepLeak 0**, false-keeps 1.8 — top of every quality
  metric *and* recall-safe.
- **Counter-consideration (recall/precision tradeoff):** for gpt-oss, ablation *lowers*
  keepRecall (0.757 → 0.629) — without grounding it pushes more true keeps to `forse`
  (reviewable, not lost — keepLeak stays 0). So grounding trades precision (more false-keeps)
  for keep-retention (fewer keeps sent to review). Which side wins depends on whether the
  operator prefers a tighter shortlist or fewer manual reviews.

### 7.1 Snippet-faithful re-test (the gating experiment) — grounding confirmed harmful

§7 ran without raw web snippets (not persisted). The gating question was whether grounding's
value would reappear once the analyst also sees snippets (production-faithful input). It does
not — **the opposite**. The `includeSnippets` flag re-gathers each company's neutral snippets
live from its resolved domain (cached per domain) so the analyst sees description + concepts
+ perimeter **+ snippets**. 10× paired, 0 failures.

Per-condition means (n=10, **snippets ON**):

| | DeepSeek grounded | DeepSeek ablated | gpt-oss grounded | **gpt-oss ablated** |
|---|---|---|---|---|
| accuracy | 0.779 ± 0.046 | 0.803 ± 0.042 | 0.724 ± 0.024 | **0.822 ± 0.010** |
| scartaRecall | 0.802 ± 0.047 | 0.815 ± 0.047 | 0.745 ± 0.038 | **0.895 ± 0.012** |
| keepRecall | 0.826 ± 0.059 | 0.857 ± 0.000 | 0.786 ± 0.075 | 0.714 ± 0.000 |
| false-keeps | 2.9 ± 1.0 | 1.6 ± 1.2 | 2.8 ± 0.8 | **1.4 ± 1.0** |
| keepLeak | 1.0 | 0.1 | 1.0 | **0.0** |

Grounding effect (grounded − ablated, paired, snippets ON):

| metric | DeepSeek-direct | gpt-oss-120b |
|---|---|---|
| accuracy Δ | −0.024 (t≈−1.1) | **−0.098 (t≈−12.5) — grounding HURTS, decisively** |
| scartaRecall Δ | −0.014 (t≈−0.6) | **−0.150 (t≈−12.2) — HURTS, decisively** |
| keepRecall Δ | −0.031 (ablated better) | +0.071 (t≈3.0, grounding helps) |
| false-keeps Δ | +1.3 (t≈2.4, grounding worse) | +1.4 (t≈3.3, grounding worse) |

**Findings (snippet-faithful):**
- **Grounding is net-harmful to BOTH models with production-faithful input.** For DeepSeek it
  is now negative-to-neutral on every metric (was neutral without snippets); for gpt-oss it is
  catastrophic on accuracy (−0.098) and scarta-recall (−0.150) at t≈−12 — as strong a signal
  as this harness produces.
- **Best configuration in the entire study: `gpt-oss-120b`, grounding OFF, snippets ON** —
  accuracy 0.822 ± 0.010, scartaRecall 0.895 ± 0.012, keepLeak 0, false-keeps 1.4. It is also
  the **most stable** config (grounding both lowers *and* destabilizes accuracy; ablated std is
  ~2-4× tighter).
- Grounding's only benefit remains a modest keepRecall lift (keeps→forse vs keeps→scarta), but
  ablated keepLeak is 0 — no true target is lost, only sent to review.
- **The §9 "snippets absent" caveat is resolved: it did not change the direction of the
  result; it sharpened it.**

---

## 8. Conclusions & recommendations

1. **Analyst model: gpt-oss-120b [implemented, staged — migration 070].** Quality tied with
   DeepSeek at ~15× the speed and ~25% lower token cost, on a different provider than the current
   Fireworks path. Migration `070` promotes Cerebras `gpt-oss-120b` to the default
   `candidate_match_analyst` model and demotes the previous default to a non-default fallback
   (DeepSeek via Fireworks stays resolvable by id; DeepSeek-direct remains the same-family
   alternative). Takes effect immediately on apply (registry resolution is live).
2. **Embedded grounding for the analyst: dropped [implemented, staged — migration 071].** The concept-match grounding is net-harmful
   — neutral-to-negative for DeepSeek, decisively negative for gpt-oss — and this held (sharpened)
   under the production-faithful snippet test (§7.1). `analyzeSectorAmbiguity` now omits the
   concept matches from the analyst payload, gated by `sector_analyst_concept_grounding` (default
   0; set 1 to restore). The analyst still gets the distilled description, strategy perimeter, and
   web snippets. The gating caveat (snippets) is now closed (§7.1). Note: embed+rerank still runs
   to drive the deterministic gate (§1.1); fully removing it from classification is a larger
   follow-up, but it is no longer in the analyst's decision input.
3. **Deterministic gate: reversed (staged).** The terminal-reject is replaced by a threshold-free
   **structural** rule in `sectorVerdict`: terminal-reject only when the top KB concept is an
   off-target distractor *and* nothing in the strategy perimeter matched — any in-perimeter signal
   escalates to the LLM. The old relative-margin (`sector_reject_rel_margin`, migration 068) leaked
   true keeps and did not generalize across sessions; the structural rule has no number to overfit
   (migration **069** removes the dead param). Takes effect on new validations only (existing
   `DeterministicVerdict`s persist until re-validation). Together with #2, UC2 moves toward "LLM
   decides from the distilled description + snippets; embeddings out of the classification loop".

### 8.1 Production validation (post re-validation, both sessions)

After applying migrations 069/070/071, deploying, and re-running "Esegui validazione" on both
sessions (live hybrid path: structural gate → gpt-oss analyst, grounding off), the persisted
production (`final`) column confirms the changes hold:

| | Liguria (23 labeled) | Nord-Est (46 labeled) |
|---|---|---|
| accuracy (all) | 0.652 | 0.630 |
| **keepLeak** | **0** | **0** (was 2 under the old relative rule) |
| accuracy (resolved only) | **0.83** (15/18) | **0.75** (27/36) |
| keepRecall (resolved keeps) | **2/2** | **5/5** |
| hard false-keeps | 1 (LINFA) | 1 (FASTCODE) |
| domain acceptance/retrieval-fail | 10/29 (34%) | 20/61 (33%) |

- **keepLeak = 0 on BOTH sessions**, including the one (Nord-Est) where the retired relative-margin
  rule leaked 2 keeps — the decisive validation of the structural reversal. Combined: **9 true keeps,
  0 leaked, 7/7 resolved keeps kept** (the 2 not-kept are domain-unresolved → forse/review, not model
  errors).
- The structural gate terminal-rejects only clear off-targets (plastic cards, POS hardware, brass
  stamping, swimwear) — all correctly scarta; everything with in-perimeter signal escalates.
- **Domain resolution is the dominant residual error** (~1/3 acceptance_fail on both) — true-scarta
  companies dumped to *forse* because the site never resolved. On resolved companies the new stack is
  strong (acc 0.75–0.83, keepLeak 0, 1 soft-ish false-keep per session). **Next lever = the domain
  resolver, not the model/gate/grounding.**

---

## 9. Caveats & open questions

- **Snippets — RESOLVED (§7.1).** The original ablation lacked raw web snippets (not persisted).
  The gating re-test re-gathered them live (`includeSnippets`, cached per domain) for a
  production-faithful input. Result: grounding's harm *increased*, not decreased — so the missing
  snippets were not masking a benefit. Residual: re-gathered snippets may have drifted since the
  original validation (web changes); identical for both arms, so fair for the comparison.
- **Analyst-on-all ≠ production hybrid.** Every number here is the solo-LLM path; the live
  pipeline gates with the deterministic verdict first. The comparison is valid for choosing a
  model/grounding, not for predicting the hybrid's live accuracy.
- **Small, scarta-heavy keep set (7 keeps).** Keep-side metrics are noisy; don't over-read
  keepRecall/keepLeak deltas. More labeled keeps (and more sessions/perimeters) would harden
  the keep-side conclusions.
- **Two perimeters only.** Both sessions are IT/MSP/cyber theses. Generalization to other
  sectors is unverified.
- **Run-to-run nondeterminism ≈ ±0.03 accuracy.** Single-run comparisons are unreliable; this
  is why every decision-grade comparison here is 10×.

---

## 10. Reproduction

**Endpoint:** `POST http://localhost:8080/api/binocolo/v1/test/sector-eval-models` (dev;
requires the backend running with auth bypass).

**Model ids** (confirm live bindings via `GET /binocolo/v1/ma/llm-options` — they change
out-of-band): DeepSeek-Fireworks `282080c5-…`, DeepSeek-direct `97e9dad0-…`,
gpt-oss-120b `136233a7-…`. **Session ids** in the header above.

**One grounded comparison (per session, both models):**
```bash
curl -s http://localhost:8080/api/binocolo/v1/test/sector-eval-models \
  -H 'content-type: application/json' -d '{
    "sessionIds": ["84781682-98b6-442d-877f-a1d9b72a8b9e"],
    "modelIds": ["97e9dad0-...","136233a7-..."]
  }' | jq .
```

**Ablation (drop the grounding):** add `"dropConceptMatches": true`.

**10× drivers + aggregators** (in `artifacts/claude/`, gitignored):
`variance_driver.sh` + `variance_agg.mjs` (§5); `ablation_driver.sh` + `ablation_agg.mjs`
(§7); `ablation_snip_driver.sh` (snippet-faithful, §7.1) aggregated via
`node ablation_agg.mjs 10 <abs-path>/ablation_snip`; `grounded_h2h.mjs` (§6). Run a driver
with `bash <abs-path>/<driver>.sh 10`, then the matching aggregator with
`node <abs-path>/<agg>.mjs 10`.

**Reversal code:** structural reject in `sectorVerdict` (`ma_sector_classification.go`);
cleanup migration `deploy/migrations/069_binocolo_uc2_sector_structural_reject.sql`.

**Code:** endpoint + harness in `backend/internal/binocolo/ma_sector_model_compare.go`;
analyst core `runSectorAnalyst` (model-parameterized) in `ma_sector_classification.go`;
handler/route in `handler.go`.
