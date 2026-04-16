resource "authzx_policy" "editors_can_edit" {
  application_id = authzx_application.example.id
  name           = "editors-can-edit"
  description    = "Editors can read and write documents"
  effect         = "ALLOW"
  actions        = ["read", "write"]
  resource_type  = "document"
  priority       = 50
}
