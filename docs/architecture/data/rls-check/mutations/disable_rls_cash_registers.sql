-- Mutacion: org.cash_registers sin RLS (NO FORCE y DISABLE): sin tenant ni usuario se leen las cajas de todas las empresas.
-- Uso: MUTATION=disable_rls_cash_registers sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE org.cash_registers NO FORCE ROW LEVEL SECURITY;
ALTER TABLE org.cash_registers DISABLE ROW LEVEL SECURITY;
