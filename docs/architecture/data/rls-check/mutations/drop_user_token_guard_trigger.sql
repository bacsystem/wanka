-- Mutacion: quita el trigger de iam.user_tokens: created_at y consumed_at dejan de fijarse en el servidor y consumed_at se reabre (#231).
-- Uso: MUTATION=drop_user_token_guard_trigger sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP TRIGGER user_tokens_guard_writes ON iam.user_tokens;
