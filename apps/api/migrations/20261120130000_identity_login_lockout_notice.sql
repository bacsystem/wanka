-- +goose Up
-- Aviso del bloqueo del login (#24, ADR 0004 nota del 2026-11-19 y ADR 0006): el job identidad.login_lockout_notice corre
-- como wanka_worker y no recibe el correo en sus argumentos (la tabla de jobs no lleva datos personales), así que lo lee
-- de iam.users. El GRANT es por columna: solo id (para el WHERE) y email, nunca password_hash ni los contadores. Sin
-- política, rol ni función nueva; iam.users no tiene RLS (identidad global).
GRANT SELECT (id, email) ON iam.users TO wanka_worker;
