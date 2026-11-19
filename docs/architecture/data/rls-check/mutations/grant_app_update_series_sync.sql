-- Mutacion: da a wanka_app UPDATE de provider_synced_at en org.document_series (ADR 0013: solo el worker marca la sincronizacion).
-- Uso: MUTATION=grant_app_update_series_sync sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT UPDATE (provider_synced_at) ON org.document_series TO wanka_app;
