# Changelog

## v0.1.0 — 2026-04-16

Initial public release of the AuthzX Terraform provider.

### Resources

- `authzx_application`
- `authzx_resource_type`
- `authzx_resource`
- `authzx_subject`
- `authzx_role`
- `authzx_group`
- `authzx_policy`
- `authzx_policy_assignment`
- `authzx_role_assignment`

All resources support Create, Read, Update, Delete, and Import.

### Authentication

- API key via provider `api_key` attribute or `AUTHZX_API_KEY` environment variable.
- Optional `base_url` for self-hosted AuthzX deployments.
