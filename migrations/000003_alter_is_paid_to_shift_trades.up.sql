-- 既存 NULL を埋める
UPDATE shift_trades SET is_paid = FALSE WHERE is_paid IS NULL;

-- NULL を禁止
ALTER TABLE shift_trades
    ALTER COLUMN is_paid SET NOT NULL;

-- 念のためデフォルトも明示（既にあるならOK）
ALTER TABLE shift_trades
    ALTER COLUMN is_paid SET DEFAULT FALSE;