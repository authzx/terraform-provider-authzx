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
  type      = string
  sensitive = false
}

variable "vengtoo_client_secret" {
  type      = string
  sensitive = true
}

# Create an application
resource "vengtoo_application" "crm" {
  name        = "CRM Platform"
  description = "Customer relationship management"
}

# Define a resource type with actions
resource "vengtoo_resource_type" "document" {
  application_id = vengtoo_application.crm.id
  name           = "document"
  description    = "Documents and files"
  actions        = ["read", "write", "delete", "share"]
}

# Create roles
resource "vengtoo_role" "admin" {
  application_id = vengtoo_application.crm.id
  name           = "admin"
  description    = "Full access"
}

resource "vengtoo_role" "editor" {
  application_id = vengtoo_application.crm.id
  name           = "editor"
  description    = "Read and write access"
}

resource "vengtoo_role" "viewer" {
  application_id = vengtoo_application.crm.id
  name           = "viewer"
  description    = "Read-only access"
}

# Create subjects
resource "vengtoo_subject" "alice" {
  application_id = vengtoo_application.crm.id
  name           = "Alice"
  type           = "user"
}

resource "vengtoo_subject" "bob" {
  application_id = vengtoo_application.crm.id
  name           = "Bob"
  type           = "user"
}

# Create policies
resource "vengtoo_policy" "admin_full_access" {
  application_id = vengtoo_application.crm.id
  name           = "admin-full-access"
  description    = "Admins can do everything"
  effect         = "ALLOW"
  actions        = ["read", "write", "delete", "share"]
  resource_type  = "document"
}

resource "vengtoo_policy" "editor_read_write" {
  application_id = vengtoo_application.crm.id
  name           = "editor-read-write"
  description    = "Editors can read and write"
  effect         = "ALLOW"
  actions        = ["read", "write"]
  resource_type  = "document"
}

resource "vengtoo_policy" "viewer_read_only" {
  application_id = vengtoo_application.crm.id
  name           = "viewer-read-only"
  description    = "Viewers can only read"
  effect         = "ALLOW"
  actions        = ["read"]
  resource_type  = "document"
}

# App-wide policy — protects all resources in the app
resource "vengtoo_policy" "app_wide_read" {
  application_id  = vengtoo_application.crm.id
  name            = "app-wide-read"
  description     = "Allow read on all resources in app"
  effect          = "ALLOW"
  priority        = 40
  actions         = ["read"]
  application_ids = [vengtoo_application.crm.id]
}
