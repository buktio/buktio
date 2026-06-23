DROP TABLE IF EXISTS signup_attempts;
DROP TABLE IF EXISTS email_verifications;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
