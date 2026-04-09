import { useMemo } from 'react';
import type { ProductVariant } from '../api/types';
import { useUpdateProduct } from '../api/queries';
import styles from './ProductGroupRadio.module.css';

interface ProductGroupRadioProps {
  products: ProductVariant[];
  quoteId: number;
  rowId: number;
}

export function ProductGroupRadio({ products, quoteId, rowId }: ProductGroupRadioProps) {
  const updateProduct = useUpdateProduct();

  // Group products by group_name
  const groups = useMemo(() => {
    const map = new Map<string, ProductVariant[]>();
    for (const p of products) {
      const list = map.get(p.group_name) ?? [];
      list.push(p);
      map.set(p.group_name, list);
    }
    return Array.from(map.entries()).map(([name, items]) => ({
      name,
      items,
      required: items.some(i => i.required),
    }));
  }, [products]);

  const handleSelect = (product: ProductVariant) => {
    updateProduct.mutate({
      quoteId, rowId, productId: product.id,
      data: { included: true, quantity: product.quantity || 1 },
    });
  };

  const handleQuantity = (product: ProductVariant, qty: number) => {
    updateProduct.mutate({
      quoteId, rowId, productId: product.id,
      data: { quantity: qty },
    });
  };

  return (
    <div>
      {groups.map(g => (
        <div key={g.name} className={`${styles.group} ${g.required ? styles.required : ''}`}>
          <div className={styles.groupHeader}>{g.name}</div>
          {g.items.map(p => (
            <div key={p.id} className={styles.variant}>
              <input
                className={styles.radioInput}
                type="radio"
                name={`group-${rowId}-${g.name}`}
                checked={p.included}
                onChange={() => handleSelect(p)}
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
                  min={p.minimum}
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
