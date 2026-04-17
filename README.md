# AuthzX Terraform Provider

Terraform provider for [AuthzX](https://authzx.com) — manage applications, resources, subjects, roles, groups, and policies as infrastructure as code.

Requires Terraform 1.0+.

## Install

```hcl
terraform {
  required_providers {
    authzx = {
      source  = "authzx/authzx"
      version = "~> 0.2"
    }
  }
}

provider "authzx" {
  # Credentials read from AUTHZX_CLIENT_ID / AUTHZX_CLIENT_SECRET env vars.
}
```

Run `terraform init` to download the provider.

## Authentication

The provider uses the OAuth 2.0 Client Credentials flow. Create an OAuth client from the AuthzX console (**Settings → API → OAuth Clients**). Client secrets are prefixed with `azx_cs_`.

The simplest setup — export env vars and leave the provider block empty:

```bash
export AUTHZX_CLIENT_ID="client_..."
export AUTHZX_CLIENT_SECRET="azx_cs_..."
```

Or set them explicitly in the provider block:

```hcl
provider "authzx" {
  client_id     = "client_..."
  client_secret = "azx_cs_..."
  # endpoint    = "https://api.authzx.com"   # optional; or AUTHZX_ENDPOINT env var
}
```

The provider exchanges credentials for a short-lived access token at startup and refreshes automatically before expiry.

## Quick example

```hcl
resource "authzx_application" "app" {
  name        = "Documents"
  description = "Document management app"
}

resource "authzx_resource_type" "document" {
  application_id = authzx_application.app.id
  name           = "document"
  actions        = ["read", "write", "delete", "share"]
}

resource "authzx_subject" "alice" {
  application_id = authzx_application.app.id
  name           = "Alice"
  type           = "user"
}

resource "authzx_role" "editor" {
  application_id = authzx_application.app.id
  name           = "editor"
  description    = "Can read and write documents"
}

resource "authzx_resource" "wiki" {
  application_id = authzx_application.app.id
  name           = "Engineering Wiki"
  type           = authzx_resource_type.document.id
}

resource "authzx_policy" "editors_can_edit" {
  application_id = authzx_application.app.id
  name           = "editors-can-edit"
  description    = "Editors can read and write the wiki"
  effect         = "ALLOW"
  priority       = 50
  resources = [
    {
      resource_id = authzx_resource.wiki.id
      actions     = ["read", "write"]
    },
  ]
}

resource "authzx_policy_assignment" "editors_can_edit" {
  policy_id   = authzx_policy.editors_can_edit.id
  entity_type = "role"
  entity_id   = authzx_role.editor.id
}

resource "authzx_role_assignment" "alice_editor" {
  subject_id = authzx_subject.alice.id
  role_id    = authzx_role.editor.id
}
```

See [`examples/`](./examples) for per-resource snippets.

## Resources

| Resource | Description |
|----------|-------------|
| [`authzx_application`](https://registry.terraform.io/providers/authzx/authzx/latest/docs/resources/application) | Container for an authorization model (resource types, policies, subjects, roles). |
| [`authzx_resource_type`](https://registry.terraform.io/providers/authzx/authzx/latest/docs/resources/resource_type) | Resource type with a set of available actions. |
| [`authzx_resource`](https://registry.terraform.io/providers/authzx/authzx/latest/docs/resources/resource) | Instance of a resource type that policies reference. |
| [`authzx_subject`](https://registry.terraform.io/providers/authzx/authzx/latest/docs/resources/subject) | User, service, or device that can be granted access. |
| [`authzx_role`](https://registry.terraform.io/providers/authzx/authzx/latest/docs/resources/role) | Named collection of policies for assigning to subjects or groups. |
| [`authzx_group`](https://registry.terraform.io/providers/authzx/authzx/latest/docs/resources/group) | Tenant-wide collection of subjects for bulk role/policy assignment. |
| [`authzx_policy`](https://registry.terraform.io/providers/authzx/authzx/latest/docs/resources/policy) | ALLOW/DENY rule with priority and conditions. |
| [`authzx_policy_assignment`](https://registry.terraform.io/providers/authzx/authzx/latest/docs/resources/policy_assignment) | Attach a policy to a role, subject, or group. |
| [`authzx_role_assignment`](https://registry.terraform.io/providers/authzx/authzx/latest/docs/resources/role_assignment) | Attach a role to a subject. |

## Import

All resources support import by ID. Single-ID resources take the resource's UUID. Assignments take composite IDs.

```bash
# Single-ID resources
terraform import authzx_application.app      <application-id>
terraform import authzx_resource_type.doc    <resource-type-id>
terraform import authzx_resource.wiki        <resource-id>
terraform import authzx_subject.alice        <subject-id>
terraform import authzx_role.editor          <role-id>
terraform import authzx_group.engineering    <group-id>
terraform import authzx_policy.my_policy     <policy-id>

# Composite-ID resources
terraform import authzx_policy_assignment.x  <entity_type>:<entity_id>:<policy_id>
terraform import authzx_role_assignment.y    <subject_id>:<role_id>
```

## Requirements

- Terraform `>= 1.0`
- Go `>= 1.23` (for contributors only)

## Development

```bash
# Build
go build ./...

# Regenerate docs from provider schema
make docs

# Install locally for testing (installs to ~/.terraform.d/plugins/)
make install

# Run unit tests
make test
```

See [`examples/`](./examples) for usage snippets and [`test-live/`](./test-live) (gitignored) for end-to-end test fixtures.

## License

MPL-2.0. See [LICENSE](./LICENSE).
