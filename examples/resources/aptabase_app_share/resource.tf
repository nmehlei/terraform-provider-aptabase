resource "aptabase_app_share" "example" {
  app_id = aptabase_app.example.id
  email  = "teammate@example.com"
}
