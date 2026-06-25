# Vengtoo Terraform Provider

Terraform provider for [Vengtoo](https://vengtoo.com) — manage applications, resources, subjects, roles, and policies as infrastructure as code.

Requires Terraform 1.0+.

## Install

```hcl
terraform {
  required_providers {
    vengtoo = {
      source  = "vengtoo/vengtoo"
      version = "~> 1.2"
    }
  }
}

provider "vengtoo" {
  # Credentials read from VENGTOO_CLIENT_ID / VENGTOO_CLIENT_SECRET env vars.
}
```

Run `terraform init` to download the provider.

## Authentication

The provider uses the OAuth 2.0 Client Credentials flow. Create an OAuth client from the Vengtoo console (**Settings → API → OAuth Clients**). Client secrets are prefixed with `azx_cs_`.

The simplest setup — export env vars and leave the provider block empty:

```bash
export VENGTOO_CLIENT_ID="client_..."
export VENGTOO_CLIENT_SECRET="azx_cs_..."
```

Or set them explicitly in the provider block:

```hcl
provider "vengtoo" {
  client_id     = "client_..."
  client_secret = "azx_cs_..."
  # endpoint    = "https://api.vengtoo.com"   # optional; or VENGTOO_ENDPOINT env var
}
```

The provider exchanges credentials for a short-lived access token at startup and refreshes automatically before expiry.

## Quick example

```hcl
resource "vengtoo_application" "app" {
  name        = "Documents"
  description = "Document management app"
}

resource "vengtoo_resource_type" "document" {
  application_id = vengtoo_application.app.id
  name           = "document"
  actions        = ["read", "write", "delete", "share"]
}

resource "vengtoo_subject" "alice" {
  application_id = vengtoo_application.app.id
  name           = "Alice"
  type           = "user"
}

resource "vengtoo_role" "editor" {
  application_id = vengtoo_application.app.id
  name           = "editor"
  description    = "Can read and write documents"
}

resource "vengtoo_resource" "wiki" {
  application_id = vengtoo_application.app.id
  name           = "Engineering Wiki"
  type           = vengtoo_resource_type.document.id
}

resource "vengtoo_policy" "editors_can_edit" {
  name           = "editors-can-edit"
  description    = "Editors can read and write the wiki"
  effect         = "ALLOW"
  priority       = 50
  resources = [
    {
      resource_id = vengtoo_resource.wiki.id
      actions     = ["read", "write"]
    },
  ]
}

resource "vengtoo_policy_assignment" "editors_can_edit" {
  policy_id   = vengtoo_policy.editors_can_edit.id
  entity_type = "role"
  entity_id   = vengtoo_role.editor.id
}

resource "vengtoo_role_assignment" "alice_editor" {
  subject_id = vengtoo_subject.alice.id
  role_id    = vengtoo_role.editor.id
}
```

See [`examples/`](./examples) for per-resource snippets.

## Resources

| Resource | Description |
|----------|-------------|
| [`vengtoo_application`](https://registry.terraform.io/providers/vengtoo/vengtoo/latest/docs/resources/application) | Container for an authorization model (resource types, policies, subjects, roles). |
| [`vengtoo_resource_type`](https://registry.terraform.io/providers/vengtoo/vengtoo/latest/docs/resources/resource_type) | Resource type with a set of available actions. |
| [`vengtoo_resource`](https://registry.terraform.io/providers/vengtoo/vengtoo/latest/docs/resources/resource) | Instance of a resource type that policies reference. |
| [`vengtoo_subject`](https://registry.terraform.io/providers/vengtoo/vengtoo/latest/docs/resources/subject) | User, service, or device that can be granted access. |
| [`vengtoo_role`](https://registry.terraform.io/providers/vengtoo/vengtoo/latest/docs/resources/role) | Named collection of policies for assigning to subjects. |
| [`vengtoo_policy`](https://registry.terraform.io/providers/vengtoo/vengtoo/latest/docs/resources/policy) | ALLOW/DENY rule with priority and conditions. |
| [`vengtoo_policy_assignment`](https://registry.terraform.io/providers/vengtoo/vengtoo/latest/docs/resources/policy_assignment) | Attach a policy to a role or subject. |
| [`vengtoo_role_assignment`](https://registry.terraform.io/providers/vengtoo/vengtoo/latest/docs/resources/role_assignment) | Attach a role to a subject. |

## Import

All resources support import by ID. Single-ID resources take the resource's UUID. Assignments take composite IDs.

```bash
# Single-ID resources
terraform import vengtoo_application.app      <application-id>
terraform import vengtoo_resource_type.doc    <resource-type-id>
terraform import vengtoo_resource.wiki        <resource-id>
terraform import vengtoo_subject.alice        <subject-id>
terraform import vengtoo_role.editor          <role-id>
terraform import vengtoo_policy.my_policy     <policy-id>

# Composite-ID resources
terraform import vengtoo_policy_assignment.x  <entity_type>:<entity_id>:<policy_id>
terraform import vengtoo_role_assignment.y    <subject_id>:<role_id>
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
