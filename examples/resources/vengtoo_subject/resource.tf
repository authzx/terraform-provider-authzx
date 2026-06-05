resource "vengtoo_subject" "alice" {
  application_id = vengtoo_application.example.id
  name           = "Alice"
  type           = "user"
}
