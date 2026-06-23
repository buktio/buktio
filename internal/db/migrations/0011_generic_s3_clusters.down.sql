-- Reinstating NOT NULL: first neutralize generic-S3 rows that legitimately have a
-- NULL admin token (they have no admin plane) so the constraint can be re-applied
-- without leaving the migration dirty.
UPDATE storage_clusters SET admin_token_enc = '\x'::bytea WHERE admin_token_enc IS NULL;
ALTER TABLE storage_clusters ALTER COLUMN admin_token_enc SET NOT NULL;
