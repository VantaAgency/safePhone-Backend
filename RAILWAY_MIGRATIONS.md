# Railway Migration Runbook

This project uses the same PostgreSQL database for:

- the SafePhone Go backend schema
- the Better Auth tables used by `safephone-app`

Because of that, the migration order matters on a fresh Railway database.

## Required order

1. Run Better Auth migrations from `safephone-app`
2. Run backend SQL migrations from `safephone-backend`

If you run backend migrations first on a brand new database, migration `000015_add_role_to_better_auth_user` can fail because the Better Auth `"user"` table does not exist yet.

## 1. Set the Railway database URL

Use the public Railway Postgres URL when running commands from your local machine.

Example:

```bash
export DATABASE_URL='postgresql://postgres:YOUR_PASSWORD@YOUR_PUBLIC_RAILWAY_HOST:YOUR_PORT/railway'
```

You can also inline it per command instead of exporting it globally.

## 2. Run Better Auth migrations first

From the frontend app:

```bash
cd /home/cherif/Documents/personal_project/SafePhone/safephone-app
DATABASE_URL="$DATABASE_URL" npx @better-auth/cli@latest migrate --config src/lib/auth/server.ts --yes
```

This creates the Better Auth tables, including the `"user"` table expected later by backend migration `000015`.

## 3. Run backend migrations

From the backend app:

```bash
cd /home/cherif/Documents/personal_project/SafePhone/safephone-backend
DATABASE_URL="$DATABASE_URL" make migrate-up
```

If you prefer to bypass `make`:

```bash
/home/cherif/go/bin/migrate -path migrations -database "$DATABASE_URL" up
```

## Dirty migration recovery

If backend migration `000015` already failed before Better Auth tables existed, the database may be marked dirty.

In that case:

1. Run the Better Auth migration first
2. Force the backend migration state back to `14`
3. Re-run backend migrations

Commands:

```bash
cd /home/cherif/Documents/personal_project/SafePhone/safephone-app
DATABASE_URL="$DATABASE_URL" npx @better-auth/cli@latest migrate --config src/lib/auth/server.ts --yes
```

```bash
cd /home/cherif/Documents/personal_project/SafePhone/safephone-backend
/home/cherif/go/bin/migrate -path migrations -database "$DATABASE_URL" force 14
/home/cherif/go/bin/migrate -path migrations -database "$DATABASE_URL" up
```

`force 14` is the correct recovery point for this specific failure because `000015` is the migration that depends on the Better Auth `"user"` table.

## Roll back the last backend migration

```bash
cd /home/cherif/Documents/personal_project/SafePhone/safephone-backend
DATABASE_URL="$DATABASE_URL" make migrate-down
```

Or directly:

```bash
/home/cherif/go/bin/migrate -path migrations -database "$DATABASE_URL" down 1
```

## Full example sequence for a fresh Railway database

```bash
export DATABASE_URL='postgresql://postgres:YOUR_PASSWORD@YOUR_PUBLIC_RAILWAY_HOST:YOUR_PORT/railway'

cd /home/cherif/Documents/personal_project/SafePhone/safephone-app
DATABASE_URL="$DATABASE_URL" npx @better-auth/cli@latest migrate --config src/lib/auth/server.ts --yes

cd /home/cherif/Documents/personal_project/SafePhone/safephone-backend
DATABASE_URL="$DATABASE_URL" make migrate-up
```

## Notes

- Do not use `postgres.railway.internal` from your laptop; that hostname is for Railway's private network.
- `safephone-backend/Makefile` includes `.env`, so inline `DATABASE_URL=... make migrate-up` is safer than relying on a previously exported variable.
- On Railway deployment day, the clean order is:
  1. provision Postgres
  2. run Better Auth migration
  3. run backend migrations
  4. deploy frontend
  5. deploy backend
