-- Password hashes removed by the up migration cannot be recovered.
ALTER TABLE users ADD COLUMN password_hash VARCHAR(255) NOT NULL DEFAULT '';
