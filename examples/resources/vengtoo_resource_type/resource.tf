resource "vengtoo_resource_type" "document" {
  name        = "document"
  description = "Documents and files"
  actions     = ["read", "write", "delete", "share"]
}
