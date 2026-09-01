## query estrazione RDA dal database ARAK

```sql
 SELECT po.id,
    po.type,
    po.project,
    po.description,
    po.note,
    po.object,
    po.requester_id,
    po.code,
    po.payment_method,
    po.budget_id,
    po.cost_center,
    po.budget_user_id,
    po.provider_id,
    po.currency,
    po.leasing,
    po.total_price,
    po.state,
    po.created_document,
    po.created,
    po.updated,
    po.deleted,
    po.budget_increment_id,
    po.subtracted_from_budget,
    po.provider_offer_date,
    po.provider_offer_code,
    po.reference_warehouse,
    po.advance_payment,
    p.company_name AS provider_company_name,
    p.erp_id,
    p.state AS provider_state,
    b.name AS budget_name,
    b.year AS budget_year,
    u.email AS requester_email,
    pm.code AS payment_method_code,
    pm.description AS payment_method_description,
    ( SELECT min(a.level) AS min
           FROM rda.approval a
          WHERE a.order_id = po.id AND po.state::text = 'PENDING_APPROVAL'::text AND NOT (EXISTS ( SELECT 1
                   FROM rda.approval a2
                  WHERE a2.order_id = po.id AND a2.level = a.level AND (a2.state::text = ANY (ARRAY['APPROVED'::character varying, 'REJECTED'::character varying]::text[]))))) AS current_approval_level
   FROM rda.purchase_order po
     LEFT JOIN provider_qualifications.provider p ON p.id = po.provider_id
     LEFT JOIN budgets.budget b ON b.id = po.budget_id
     LEFT JOIN users_int."user" u ON u.id = po.requester_id
     LEFT JOIN provider_qualifications.payment_method pm ON pm.code = po.payment_method
     LEFT JOIN rda.reference_warehouse wh ON wh.name::text = po.reference_warehouse::text
   WHERE po."state" not in ('DRAFT','CANCELED');
```


## query estrazione RDA con righe dal database ARAK

```sql
 SELECT po.id,
    po.type,
    po.project,
    po.description,
    po.note,
    po.object,
    po.requester_id,
    po.code,
    po.payment_method,
    po.budget_id,
    po.cost_center,
    po.budget_user_id,
    po.provider_id,
    po.currency,
    po.leasing,
    po.total_price,
    po.state,
    po.created_document,
    po.created,
    po.updated,
    po.deleted,
    po.budget_increment_id,
    po.subtracted_from_budget,
    po.provider_offer_date,
    po.provider_offer_code,
    po.reference_warehouse,
    po.advance_payment,
    p.company_name AS provider_company_name,
    p.erp_id,
    p.state AS provider_state,
    b.name AS budget_name,
    b.year AS budget_year,
    u.email AS requester_email,
    pm.code AS payment_method_code,
    pm.description AS payment_method_description,
    ( SELECT min(a.level) AS min
           FROM rda.approval a
          WHERE a.order_id = po.id AND po.state::text = 'PENDING_APPROVAL'::text AND NOT (EXISTS ( SELECT 1
                   FROM rda.approval a2
                  WHERE a2.order_id = po.id AND a2.level = a.level AND (a2.state::text = ANY (ARRAY['APPROVED'::character varying, 'REJECTED'::character varying]::text[]))))) AS current_approval_level,
    por.id as row_id, por.product_code, por."type",              
    por.qty,por.nrc,por.mrc,por.product_code,por.product_description,
    por.description,por.total,por.price 
   FROM rda.purchase_order po
     LEFT JOIN provider_qualifications.provider p ON p.id = po.provider_id
     LEFT JOIN budgets.budget b ON b.id = po.budget_id
     LEFT JOIN users_int."user" u ON u.id = po.requester_id
     LEFT JOIN provider_qualifications.payment_method pm ON pm.code = po.payment_method
     LEFT JOIN rda.reference_warehouse wh ON wh.name::text = po.reference_warehouse::text
     LEFT JOIN rda.purchase_order_row por on por.order_id = po.id 
   WHERE po."state" not in ('DRAFT','CANCELED');
```