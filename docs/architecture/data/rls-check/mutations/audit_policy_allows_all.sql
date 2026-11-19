-- Mutacion: la politica de audit.events deja pasar todo (A veria los eventos de B).
-- Uso: MUTATION=audit_policy_allows_all sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP POLICY tenant_isolation ON audit.events;
CREATE POLICY tenant_isolation ON audit.events USING (true) WITH CHECK (true);
