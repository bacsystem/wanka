-- Mutacion: la PK de ops.idempotency_keys vuelve a ser (tenant_id, key): dos usuarios del mismo tenant compartirian la clave
-- y uno recibiria la respuesta guardada del otro (conventions §3).
-- Uso: MUTATION=idempotency_pk_without_principal sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE ops.idempotency_keys DROP CONSTRAINT idempotency_keys_pkey;
ALTER TABLE ops.idempotency_keys ADD PRIMARY KEY (tenant_id, key);
