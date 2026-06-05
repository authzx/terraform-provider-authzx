resource "vengtoo_role" "editor" {
  application_id = vengtoo_application.example.id
  name           = "editor"
  description    = "Can read and write documents"
}
