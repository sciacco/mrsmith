# AFC Tools

Knowledge entries specific to `apps/afc-tools`.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### AFC Tools Order PDF Missing in Arxivar Surfaces as `ARX_DOC_NUMBER_NOT_FOUND`

- Context: `GET /api/afc-tools/v1/orders/{orderId}/pdf`, which proxies the Mistra/gw-int order PDF endpoint used by the AFC Tools XConnect orders view.
- Discovery: when the upstream order document has not yet landed in Arxivar, the external gateway can return HTTP `500` with JSON body `{"message":"ARX_DOC_NUMBER_NOT_FOUND"}` instead of a cleaner 404-style missing-resource response.
- Practical rule: mrsmith should normalize this exact upstream signal to an app-level “PDF not ready yet” state for the AFC Tools order-PDF flow, rather than surfacing it as a generic technical failure. Do not generalize other upstream 500s into the same UX state without an equally specific domain signal.
- Evidence: direct gateway call on 2026-04-19 to `gw-int /orders/v1/order/pdf/301`; AFC domain interpretation that `ARX` refers to Arxivar, the documental system.
- Used by: `apps/afc-tools` XConnect order PDF download flow and `backend/internal/afctools/gateway.go`.
- Open questions: whether other gw-int PDF/document endpoints use the same Arxivar-coded missing-document pattern and should be normalized separately.
