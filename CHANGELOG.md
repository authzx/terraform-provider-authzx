# Changelog

## v0.2.0 — 2026-04-18

### Breaking changes

- Authentication switched from API keys to OAuth 2.0 Client Credentials. Update your provider block to use `client_id` / `client_secret` instead of `api_key`. Create an OAuth client at **Settings → API → OAuth Clients** in the AuthzX console.

### Added

- `client_id` (String) provider attribute, or `AUTHZX_CLIENT_ID` env var.
- `client_secret` (String, Sensitive) provider attribute, or `AUTHZX_CLIENT_SECRET` env var.
- `endpoint` (String) provider attribute, or `AUTHZX_ENDPOINT` env var. Defaults to `https://api.authzx.com`.
- Automatic token refresh before expiry and on 401 responses.

### Removed

- `api_key` provider attribute and `AUTHZX_API_KEY` env var.
- `base_url` provider attribute (renamed to `endpoint`).

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
