-- Mutacion: quita a wanka_worker el UPDATE de provider_conflict_at. Sin el, el congelado de una serie da permission denied.
-- Uso: MUTATION=revoke_worker_series_sync sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
REVOKE UPDATE (provider_conflict_at) ON org.document_series FROM wanka_worker;
