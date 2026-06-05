resource "vengtoo_resource_type" "document" {
  application_id = vengtoo_application.example.id
  name           = "document"
  description    = "Documents and files"
  actions        = ["read", "write", "delete", "share"]
}
