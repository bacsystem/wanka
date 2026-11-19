-- Mutacion: quita el trigger que hace irreversible el congelado de una serie (ADR 0013). Sin el, wanka_app podria levantarlo.
-- Uso: MUTATION=drop_series_conflict_guard sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP TRIGGER document_series_guard_conflict ON org.document_series;
