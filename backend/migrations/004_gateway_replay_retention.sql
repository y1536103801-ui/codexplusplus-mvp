CREATE INDEX IF NOT EXISTS idx_idempotency_records_updated_at
  ON idempotency_records(updated_at);
