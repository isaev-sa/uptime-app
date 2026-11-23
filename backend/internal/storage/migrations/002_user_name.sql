ALTER TABLE users ADD COLUMN name TEXT;
UPDATE users SET name = left(split_part(email, '@', 1), 100);
ALTER TABLE users ALTER COLUMN name SET NOT NULL;
