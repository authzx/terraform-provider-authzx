resource "vengtoo_resource_type" "document" {
  name    = "document"
  actions = ["read", "write"]
}

resource "vengtoo_policy" "editors_can_edit" {
  name        = "editors-can-edit"
  description = "Editors can read and write documents"
  effect      = "ALLOW"

  resource_types = [{
    resource_type_id = vengtoo_resource_type.document.id
    actions          = ["read", "write"]
  }]
}
