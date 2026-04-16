resource "authzx_subject" "alice" {
  application_id = authzx_application.example.id
  name           = "Alice"
  type           = "user"
}
