data "aptabase_apps" "all" {}

output "app_ids" {
  value = [for app in data.aptabase_apps.all.apps : app.id]
}
