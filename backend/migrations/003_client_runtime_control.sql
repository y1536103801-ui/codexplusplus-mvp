ALTER TABLE client_access_keys
  ADD COLUMN IF NOT EXISTS device_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_client_access_keys_device_status
  ON client_access_keys(device_id, status)
  WHERE device_id <> '';
