-- Remove foreign key constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_role;

-- Drop index
DROP INDEX IF EXISTS idx_users_role_id;

-- Remove role_id column from users table
ALTER TABLE users DROP COLUMN IF EXISTS role_id;

-- Drop roles table (this will fail if there are foreign key references, 
-- but that's expected in a down migration)
DROP TABLE IF EXISTS roles;

