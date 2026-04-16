resource "authzx_role" "editor" {
  application_id = authzx_application.example.id
  name           = "editor"
  description    = "Can read and write documents"
}
