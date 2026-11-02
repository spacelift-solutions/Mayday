terraform {
  required_version = ">= 1.11.0"
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
variable "monitor_callback_url" {
  type        = string
  description = "Public HTTPS endpoint routed to the monitor's POST /api/flows/result handler."
  validation {
    condition     = can(regex("^https://[A-Za-z0-9][A-Za-z0-9.-]+(:443)?/api/flows/result$", var.monitor_callback_url)) && !strcontains(var.monitor_callback_url, ".invalid/")
    error_message = "Set a reachable HTTPS hostname with path /api/flows/result and no credentials, query, or fragment."
  }
}
variable "webhook_token" {
  type      = string
  sensitive = true
  ephemeral = true
}

resource "flows_secret" "webhook" {
  project_id = var.project_id
  key        = "MAYDAY_WEBHOOK_TOKEN"
  value      = var.webhook_token
}

resource "flows_flow" "recovery" {
  project_id = var.project_id
  name       = "Mayday recovery"
  definition = replace(file("${path.module}/../recovery.yaml"), "https://mayday-callback.invalid/api/flows/result", var.monitor_callback_url)
  enabled    = true
  depends_on = [flows_secret.webhook]
}
output "flow_id" { value = flows_flow.recovery.id }
