resource "aptabase_api_key" "ci" {
  name = "ci-automation"
}

output "ci_api_key" {
  value     = aptabase_api_key.ci.key
  sensitive = true
}
