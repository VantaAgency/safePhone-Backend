-- Add role column to Better Auth's user table so it can be exposed via session additionalFields.
-- Existing users default to 'member'; promoted admins/partners will be synced on next login.
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'member';
