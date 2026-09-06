# Energia in DC

### Customer kW Reports Follow the Daily-Summary Source Aggregation

- Context: customer / room / rack power reports in Energia in DC.
- Discovery: the daily-summary materialization supplied in issue #184 uses positive readings only, the maximum per socket and civil hour, the sum of sockets per rack/hour, then the mean of observed hourly rack totals per day. Conversion is **230 V × amperes / 1000**, not the 225 V conversion used by the existing instantaneous rack helper. Apply the selected power factor without intermediate rounding.
- Practical rule: scope by Grappa internal customer ID (`racks.id_anagrafica`) and active racks, without variable-billing filters. Aggregate raw observations in SQL; sum available daily rack values for room/customer totals, then average the available daily totals separately at each level for annual reports. Do not sum independently averaged monthly rack values. Missing buckets are null, not zero. Inventory retains active racks with sockets even if no positive observations exist in the period.
- Temporal behavior: preserve the existing Energia SQL civil-date convention and half-open bounds formatted in Europe/Rome. `rack_power_readings.date` is a MySQL TIMESTAMP, so SQL formatting uses the database session timezone, as in the existing readings handlers and materialization; the Go location alone does not change that session setting.
- Evidence: [issue #184](https://github.com/sciacco/mrsmith/issues/184), `backend/internal/energiadc/handler_kw_report.go`, `config.go`, `docs/grappa/grappa_rack_power_readings.json` (existing covering index `idx_sock_amp(rack_socket_id, date, ampere)`).
- Used by: Grafici cliente, `GET /energia-dc/v1/customers/{customerId}/kw-report`.
- Verification limitation: the source query has been inspected and exercised with synthetic data, not benchmarked or verified against the shared MySQL database.
