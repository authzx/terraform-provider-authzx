terraform {
  required_providers {
    vengtoo = {
      source = "vengtoo/vengtoo"
    }
  }
}

provider "vengtoo" {
  # Credentials can also come from VENGTOO_CLIENT_ID / VENGTOO_CLIENT_SECRET env vars.
  client_id     = var.vengtoo_client_id
  client_secret = var.vengtoo_client_secret
  # endpoint = "https://api.vengtoo.com"  # default; override for dev/staging
}

variable "vengtoo_client_id" {
  type = string
}

variable "vengtoo_client_secret" {
  type      = string
  sensitive = true
}

resource "vengtoo_resource_type" "document" {
  name        = "document"
  description = "Documents and files"
  actions     = ["read", "write", "delete", "share"]
}

resource "vengtoo_role" "editor" {
  name        = "editor"
  description = "Read and write access"
}

resource "vengtoo_subject" "alice" {
  name = "Alice"
  type = "user"
}

resource "vengtoo_policy" "editor_read_write" {
  name        = "editor-read-write"
  description = "Editors can read and write documents"
  effect      = "ALLOW"

  resource_types = [{
    resource_type_id = vengtoo_resource_type.document.id
    actions          = ["read", "write"]
  }]
}

# Assign the policy directly to Alice.
resource "vengtoo_policy_assignment" "alice_editor" {
  policy_id   = vengtoo_policy.editor_read_write.id
  entity_type = "entity"
  entity_id   = vengtoo_subject.alice.id
}
