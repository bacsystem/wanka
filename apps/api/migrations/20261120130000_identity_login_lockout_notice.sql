-- +goose Up
-- Aviso del bloqueo del login (#24, ADR 0004 nota del 2026-11-19 y ADR 0006): el job identidad.login_lockout_notice corre
-- como wanka_worker y no recibe el correo en sus argumentos (la tabla de jobs no lleva datos personales), así que lo lee
-- de iam.users. El GRANT es por columna: solo id (para el WHERE) y email, nunca password_hash ni los contadores. Sin
-- política, rol ni función nueva; iam.users no tiene RLS (identidad global).
GRANT SELECT (id, email) ON iam.users TO wanka_worker;

-- Racha de fallos seguidos del login (ADR 0014, nota del 2026-11-20): cuenta todo fallo cobrado, con cualquier espaciado.
-- Cada 5 hay un bloqueo creciente y con 100 el ingreso con contraseña queda cerrado. Solo la ponen en 0 una contraseña
-- correcta, el restablecimiento (#26) y el cambio de contraseña (#30). wanka_app ya tiene UPDATE de iam.users; el worker
-- no recibe la columna (el GRANT de arriba es solo id y email).
ALTER TABLE iam.users
  ADD COLUMN login_failure_streak smallint NOT NULL DEFAULT 0
    CONSTRAINT users_login_failure_streak_range CHECK (login_failure_streak BETWEEN 0 AND 100);
