# The shared demo-dns stack owns this zone; ExternalDNS owns application aliases.
data "aws_route53_zone" "callbacks" {
  name         = "spacelift.shovel-emporium.com"
  private_zone = false
}

resource "aws_acm_certificate" "callback" {
  domain_name       = "mayday.spacelift.shovel-emporium.com"
  validation_method = "DNS"
  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_route53_record" "callback_validation" {
  for_each = {
    for option in aws_acm_certificate.callback.domain_validation_options :
    option.domain_name => option
  }
  zone_id = data.aws_route53_zone.callbacks.zone_id
  name    = each.value.resource_record_name
  type    = each.value.resource_record_type
  records = [each.value.resource_record_value]
  ttl     = 300
}

resource "aws_acm_certificate_validation" "callback" {
  certificate_arn         = aws_acm_certificate.callback.arn
  validation_record_fqdns = [for record in aws_route53_record.callback_validation : record.fqdn]
}

output "callback_certificate_arn" {
  value = aws_acm_certificate_validation.callback.certificate_arn
}
output "callback_url" {
  value = "https://mayday.spacelift.shovel-emporium.com/api/flows/result"
}
