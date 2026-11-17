\set ON_ERROR_STOP on
\echo '== 8. (wanka_webhooks) resolve_webhook_route y acceso denegado a otras tablas =='
SELECT tenant_id, webhook_secret_enc FROM ops.resolve_webhook_route('r-0123456789abcdef');
SELECT count(*) AS ruta_inexistente FROM ops.resolve_webhook_route('r-nope');
\set ON_ERROR_STOP off
SELECT * FROM ops.webhook_routes;
\set ON_ERROR_STOP on
