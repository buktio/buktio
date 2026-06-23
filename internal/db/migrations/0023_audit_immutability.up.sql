-- 0023: tamper-evident audit log (Enterprise). Each event carries a hash chain:
-- row_hash = sha256(prev_hash || canonical(row)), prev_hash = the org's previous
-- row_hash. Any edit/delete of a historical row breaks the chain, which the verify
-- routine detects. Existing rows keep NULL hashes (pre-chain); the chain starts at
-- the first row written after this migration. OSS never populates these.
ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS prev_hash bytea;
ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS row_hash  bytea;
