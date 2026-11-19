\set ON_ERROR_STOP on
\echo '== W1. (wanka_webhooks) resolve_webhook_route =='
SELECT test.assert((SELECT tenant_id = 'aaaaaaaa-0000-0000-0000-000000000001' AND webhook_secret_enc = '\xdeadbeef'::bytea
                      FROM ops.resolve_webhook_route('r-0123456789abcdef')), 'ruta resuelta a A');
SELECT test.assert((SELECT count(*) FROM ops.resolve_webhook_route('r-nope')) = 0, 'ruta inexistente: 0 filas');
SELECT test.expect_error('SELECT * FROM ops.webhook_routes', '42501', 'wanka_webhooks sin SELECT directo');
\echo '== W2. (wanka_webhooks) tablas de River: solo INSERT en ops.river_job =='
SELECT test.river_enqueue_only();
\echo '== W3. (wanka_webhooks) sin acceso a audit.events (#173) =='
SELECT test.expect_error('SELECT count(*) FROM audit.events', '42501', 'wanka_webhooks no lee audit.events');
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);   -- con tenant, para que solo el GRANT (no la RLS) lo impida
SELECT test.expect_error($$INSERT INTO audit.events (tenant_id, actor_type, action) VALUES (app.current_tenant(), 'system', 'rls.check:w')$$,
                         '42501', 'wanka_webhooks no inserta en audit.events');
ROLLBACK;
