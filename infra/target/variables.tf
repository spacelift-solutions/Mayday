variable "region" {
  type    = string
  default = "us-east-1"
}
variable "cluster_name" {
  type = string
}
variable "ssh_public_key" {
  type = string
}
variable "instance_type" {
  type    = string
  default = "t3.small"
}
variable "license_key" {
  type    = string
  default = "license/demo-license.json"
}
