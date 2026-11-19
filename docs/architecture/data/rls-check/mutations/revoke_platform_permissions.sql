-- Mutacion: quita el SELECT de wanka_platform sobre iam.permissions (un GRANT de menos respecto del ADR 0001).
-- Demuestra que el caso P5 de 03_platform.sql distingue.
-- Uso: MUTATION=revoke_platform_permissions sh run.sh   (debe fallar en el paso wanka_platform)
\set ON_ERROR_STOP on
REVOKE SELECT ON iam.permissions FROM wanka_platform;
