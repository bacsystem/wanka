-- Mutacion: tenant_isolation de ops.idempotency_keys pasa a USING (true): cada tenant veria (y repetiria) las respuestas
-- guardadas de los demas.
-- Uso: MUTATION=idempotency_open_policy sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP POLICY tenant_isolation ON ops.idempotency_keys;
CREATE POLICY tenant_isolation ON ops.idempotency_keys USING (true) WITH CHECK (true);
