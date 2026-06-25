# Changelog

## v1.2.0 — 2026-06-25

### Removed

- `vengtoo_group` resource — groups are no longer part of the Vengtoo model. Assign policies directly to subjects or via roles (`vengtoo_role_assignment`).

## v0.2.0 — 2026-04-18

### Breaking changes

- Authentication switched from API keys to OAuth 2.0 Client Credentials. Update your provider block to use `client_id` / `client_secret` instead of `api_key`. Create an OAuth client at **Settings → API → OAuth Clients** in the Vengtoo console.

### Added

- `client_id` (String) provider attribute, or `VENGTOO_CLIENT_ID` env var.
- `client_secret` (String, Sensitive) provider attribute, or `VENGTOO_CLIENT_SECRET` env var.
- `endpoint` (String) provider attribute, or `VENGTOO_ENDPOINT` env var. Defaults to `https://api.vengtoo.com`.
- Automatic token refresh before expiry and on 401 responses.

### Removed

- `api_key` provider attribute and `VENGTOO_API_KEY` env var.
- `base_url` provider attribute (renamed to `endpoint`).

## v0.1.0 — 2026-04-16

Initial public release of the Vengtoo Terraform provider.

### Resources

- `vengtoo_application`
- `vengtoo_resource_type`
- `vengtoo_resource`
- `vengtoo_subject`
- `vengtoo_role`
- `vengtoo_group`
- `vengtoo_policy`
- `vengtoo_policy_assignment`
- `vengtoo_role_assignment`

All resources support Create, Read, Update, Delete, and Import.

### Authentication

- API key via provider `api_key` attribute or `VENGTOO_API_KEY` environment variable.
- Optional `base_url` for self-hosted Vengtoo deployments.
