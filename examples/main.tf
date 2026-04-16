terraform {
  required_providers {
    authzx = {
      source = "authzx/authzx"
    }
  }
}

provider "authzx" {
  api_key = ""
  # base_url = "https://api.authzx.com"  # default
}

# Create an application
resource "authzx_application" "crm" {
  name        = "CRM Platform"
  description = "Customer relationship management"
}

# Define a resource type with actions
resource "authzx_resource_type" "document" {
  application_id = authzx_application.crm.id
  name           = "document"
  description    = "Documents and files"
  actions        = ["read", "write", "delete", "share"]
}

# Create roles
resource "authzx_role" "admin" {
  application_id = authzx_application.crm.id
  name           = "admin"
  description    = "Full access"
}

resource "authzx_role" "editor" {
  application_id = authzx_application.crm.id
  name           = "editor"
  description    = "Read and write access"
}

resource "authzx_role" "viewer" {
  application_id = authzx_application.crm.id
  name           = "viewer"
  description    = "Read-only access"
}

# Create subjects
resource "authzx_subject" "alice" {
  application_id = authzx_application.crm.id
  name           = "Alice"
  type           = "user"
}

resource "authzx_subject" "bob" {
  application_id = authzx_application.crm.id
  name           = "Bob"
  type           = "user"
}

# Create policies
resource "authzx_policy" "admin_full_access" {
  application_id = authzx_application.crm.id
  name           = "admin-full-access"
  description    = "Admins can do everything"
  effect         = "ALLOW"
  actions        = ["read", "write", "delete", "share"]
  resource_type  = "document"
}

resource "authzx_policy" "editor_read_write" {
  application_id = authzx_application.crm.id
  name           = "editor-read-write"
  description    = "Editors can read and write"
  effect         = "ALLOW"
  actions        = ["read", "write"]
  resource_type  = "document"
}

resource "authzx_policy" "viewer_read_only" {
  application_id = authzx_application.crm.id
  name           = "viewer-read-only"
  description    = "Viewers can only read"
  effect         = "ALLOW"
  actions        = ["read"]
  resource_type  = "document"
}
