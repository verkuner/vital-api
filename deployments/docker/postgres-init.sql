-- Creates the Keycloak database on first PostgreSQL startup.
-- The app database (vital_signs) is created automatically via POSTGRES_DB.
SELECT 'CREATE DATABASE keycloak'
  WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'keycloak')\gexec

GRANT ALL PRIVILEGES ON DATABASE keycloak TO vital;
