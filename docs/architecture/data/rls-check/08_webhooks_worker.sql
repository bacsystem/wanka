\set ON_ERROR_STOP on
\echo '== W1. (wanka_webhooks) resolve_webhook_route =='
SELECT test.assert((SELECT tenant_id = 'aaaaaaaa-0000-0000-0000-000000000001' AND webhook_secret_enc = '\xdeadbeef'::bytea
                      FROM ops.resolve_webhook_route('r-0123456789abcdef')), 'ruta resuelta a A');
SELECT test.assert((SELECT count(*) FROM ops.resolve_webhook_route('r-nope')) = 0, 'ruta inexistente: 0 filas');
SELECT test.expect_error('SELECT * FROM ops.webhook_routes', '42501', 'wanka_webhooks sin SELECT directo');
\echo '== W2. (wanka_webhooks) tablas de River: solo INSERT en ops.river_job =='
SELECT test.river_enqueue_only();
\echo '== W2. entitlements (#171): wanka_webhooks no tiene acceso a platform =='
SELECT test.expect_error('SELECT count(*) FROM platform.modules', '42501', 'wanka_webhooks sin acceso a las tablas de entitlements');
SELECT test.expect_error('SELECT count(*) FROM platform.tenant_usage', '42501', 'wanka_webhooks sin acceso al uso');
\echo '== W2. (wanka_webhooks) sin acceso a iam.permissions (issue #172) =='
SELECT test.expect_error('SELECT * FROM iam.permissions', '42501', 'wanka_webhooks no lee el catalogo de permisos');
