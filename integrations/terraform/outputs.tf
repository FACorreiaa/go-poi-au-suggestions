output "namespace" {
  description = "Application namespace"
  value       = kubernetes_namespace.go_ai_poi.metadata[0].name
}

output "observability_namespace" {
  description = "Observability namespace"
  value       = kubernetes_namespace.observability.metadata[0].name
}

output "postgres_service" {
  description = "PostgreSQL service name"
  value       = kubernetes_service.postgres.metadata[0].name
}

output "app_service" {
  description = "Application service name"
  value       = kubernetes_service.go_ai_poi_app.metadata[0].name
}

output "prometheus_service" {
  description = "Prometheus service name"
  value       = kubernetes_service.prometheus.metadata[0].name
}

output "grafana_service" {
  description = "Grafana service name"
  value       = kubernetes_service.grafana.metadata[0].name
}

output "loki_service" {
  description = "Loki service name"
  value       = kubernetes_service.loki.metadata[0].name
}

output "tempo_service" {
  description = "Tempo service name"
  value       = kubernetes_service.tempo.metadata[0].name
}

output "otel_collector_service" {
  description = "OpenTelemetry Collector service name"
  value       = kubernetes_service.otel_collector.metadata[0].name
}

output "ingress_urls" {
  description = "Ingress URLs"
  value = var.enable_ingress ? {
    app        = "http://${var.domain}"
    grafana    = "http://grafana.${var.domain}"
    prometheus = "http://prometheus.${var.domain}"
  } : {}
}