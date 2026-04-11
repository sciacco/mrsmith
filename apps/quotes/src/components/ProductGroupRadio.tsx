import type { DocumentType, ProductGroup, ProductVariant } from '../api/types';
import { useUpdateProduct } from '../api/queries';
import { buildProductUpdatePayload } from '../utils/quoteRules';
import styles from './ProductGroupRadio.module.css';

interface ProductGroupRadioProps {
  groups: ProductGroup[];
  quoteId: number;
  rowId: number;
  documentType: DocumentType;
}

export function ProductGroupRadio({ groups, quoteId, rowId, documentType }: ProductGroupRadioProps) {
  const updateProduct = useUpdateProduct();
  const isSpotQuote = documentType === 'TSC-ORDINE';

  const handleSelect = (product: ProductVariant, included: boolean, quantity = product.quantity) => {
    updateProduct.mutate({
      quoteId, rowId, productId: product.id,
      data: buildProductUpdatePayload(product, included, quantity, isSpotQuote),
    });
  };

  const handleQuantity = (product: ProductVariant, qty: number) => {
    handleSelect(product, true, qty);
  };

  const handleExclude = (group: ProductGroup) => {
    const fallback = group.included_product ?? group.products[0];
    if (!fallback) return;
    handleSelect(fallback, false, fallback.quantity);
  };

  return (
    <div>
      {groups.map(g => (
        <div key={g.group_name} className={`${styles.group} ${g.required ? styles.required : ''}`}>
          <div className={styles.groupHeader}>{g.group_name}</div>
          {!g.required && (
            <label className={styles.variant}>
              <input
                className={styles.radioInput}
                type="radio"
                name={`group-${rowId}-${g.group_name}`}
                checked={g.included_product === null}
                onChange={() => handleExclude(g)}
              />
              <span className={styles.variantName}>Non incluso</span>
            </label>
          )}
          {g.products.map(p => (
            <div key={p.id} className={styles.variant}>
              <input
                className={styles.radioInput}
                type="radio"
                name={`group-${rowId}-${g.group_name}`}
                checked={p.included}
                onChange={() => handleSelect(p, true)}
              />
              <span className={styles.variantName}>{p.product_name}</span>
              <span className={styles.variantPrice}>
                {p.nrc > 0 ? `NRC ${p.nrc.toFixed(2)}` : ''}
                {p.nrc > 0 && p.mrc > 0 ? ' / ' : ''}
                {p.mrc > 0 ? `MRC ${p.mrc.toFixed(2)}` : ''}
              </span>
              {p.included && (
                <input
                  className={styles.qtyInput}
                  type="number"
                  min={p.minimum > 0 ? p.minimum : 0}
                  max={p.maximum || 999}
                  value={p.quantity}
                  onChange={e => handleQuantity(p, Number(e.target.value))}
                />
              )}
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}
