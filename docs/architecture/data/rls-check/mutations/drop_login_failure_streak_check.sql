-- Mutacion: quita el CHECK de la racha de fallos del login. Sin el, la racha pasaria de 100 y el cierre dependeria solo del codigo (#24).
-- Uso: MUTATION=drop_login_failure_streak_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE iam.users DROP CONSTRAINT users_login_failure_streak_range;
