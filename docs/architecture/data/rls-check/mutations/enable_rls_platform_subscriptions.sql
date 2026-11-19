-- Mutacion: enciende RLS en platform.subscriptions (las tablas de entitlements no tienen RLS, ADR 0001: leen todas las empresas
-- con el filtro de app.current_tenant() de la consulta). Debe fallar el caso «sin RLS» de 05_app.sql.
-- Uso: MUTATION=enable_rls_platform_subscriptions sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE platform.subscriptions ENABLE ROW LEVEL SECURITY;
