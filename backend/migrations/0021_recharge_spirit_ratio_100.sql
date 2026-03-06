UPDATE recharge_products
SET
    spirit_stones = GREATEST(
        1,
        ROUND(credit_amount::numeric * 100 * (1 + COALESCE(bonus_rate, 0)))::BIGINT
    ),
    updated_at = now()
WHERE spirit_stones <> GREATEST(
    1,
    ROUND(credit_amount::numeric * 100 * (1 + COALESCE(bonus_rate, 0)))::BIGINT
);

UPDATE recharge_orders AS ro
SET
    spirit_stones = GREATEST(
        1,
        ROUND(ro.credit_amount::numeric * 100 * (1 + COALESCE(rp.bonus_rate, 0)))::BIGINT
    ),
    updated_at = now()
FROM recharge_products AS rp
WHERE
    ro.product_code = rp.code
    AND ro.status = 'pending'
    AND ro.spirit_stones <> GREATEST(
        1,
        ROUND(ro.credit_amount::numeric * 100 * (1 + COALESCE(rp.bonus_rate, 0)))::BIGINT
    );
