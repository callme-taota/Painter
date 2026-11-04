#!/usr/bin/env bash
set -euo pipefail
# psql reads passwords from its environment instead of command-line arguments.
psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" --set ON_ERROR_STOP=1 <<'SQL'
\getenv migration_password DB_MIGRATION_PASSWORD
\getenv application_password DB_APP_PASSWORD
CREATE ROLE painter_migrator LOGIN PASSWORD :'migration_password' NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE painter_app LOGIN PASSWORD :'application_password' NOSUPERUSER NOCREATEDB NOCREATEROLE;
ALTER DATABASE painter OWNER TO painter_migrator;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT CONNECT ON DATABASE painter TO painter_app;
GRANT USAGE ON SCHEMA public TO painter_app;
ALTER DEFAULT PRIVILEGES FOR ROLE painter_migrator IN SCHEMA public GRANT SELECT,INSERT,UPDATE,DELETE ON TABLES TO painter_app;
ALTER DEFAULT PRIVILEGES FOR ROLE painter_migrator IN SCHEMA public GRANT USAGE,SELECT ON SEQUENCES TO painter_app;
SQL
