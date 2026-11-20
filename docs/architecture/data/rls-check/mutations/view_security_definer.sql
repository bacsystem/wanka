-- Mutacion: iam.list_my_memberships sin security_invoker: la vista correria con los permisos de su dueno y no con los del llamador
-- (ADR 0001: la vista no es SECURITY DEFINER).
-- Uso: MUTATION=view_security_definer sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER VIEW iam.list_my_memberships SET (security_invoker = false);
