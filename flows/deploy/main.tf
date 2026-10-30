terraform {
  required_providers {
    flows = {
      source  = "spacelift-io/flows"
      version = ">= 0.7.0, < 1.0.0"
    }
  }
}
provider "flows" { endpoint = var.flows_endpoint }
variable "flows_endpoint" {
  type    = string
  default = "useflows.eu"
}
variable "project_id" { type = string }
variable "webhook_token" {
  type      = string
  sensitive = true
}

resource "flows_secret" "webhook" {
  project_id = var.project_id
  key        = "MAYDAY_WEBHOOK_TOKEN"
  value      = var.webhook_token
}

resource "flows_flow" "recovery" {
  project_id = var.project_id
  name       = "Mayday recovery"
  definition = file("${path.module}/../recovery.yaml")
  enabled    = true
  depends_on = [flows_secret.webhook]
}
output "flow_id" { value = flows_flow.recovery.id }
