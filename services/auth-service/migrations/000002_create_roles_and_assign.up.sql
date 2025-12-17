-- Create roles table
CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert the 3 default roles
INSERT INTO roles (name, description) VALUES
    ('user', 'Regular user with standard permissions'),
    ('moderator', 'User with content moderation permissions'),
    ('admin', 'Administrator with full system access')
ON CONFLICT (name) DO NOTHING;

-- Add role_id column to users table with default value (user role = 1)
ALTER TABLE users 
ADD COLUMN role_id INTEGER NOT NULL DEFAULT 1;

-- Add foreign key constraint
ALTER TABLE users 
ADD CONSTRAINT fk_users_role 
FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE RESTRICT;

-- Create index for better query performance
CREATE INDEX idx_users_role_id ON users(role_id);

-- Assign all existing users to 'user' role (role_id = 1)
-- This is already the default, but we make it explicit
UPDATE users SET role_id = 1 WHERE role_id IS NULL OR role_id = 1;

