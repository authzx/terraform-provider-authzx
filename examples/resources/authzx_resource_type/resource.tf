resource "authzx_resource_type" "document" {
  application_id = authzx_application.example.id
  name           = "document"
  description    = "Documents and files"
  actions        = ["read", "write", "delete", "share"]
}
