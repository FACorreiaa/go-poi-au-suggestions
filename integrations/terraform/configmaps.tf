# Application ConfigMap
resource "kubernetes_config_map" "app_config" {
  metadata {
    name      = "loci-server-config"
    namespace = kubernetes_namespace.go_ai_poi.metadata[0].name
  }

  data = {
    "config.yml" = templatefile("${path.module}/../config/config.yml", {
      postgres_host = "postgres-service"
      postgres_port = "5432"
      postgres_db   = var.postgres_db
    })
  }
}

# OpenTelemetry Collector ConfigMap
resource "kubernetes_config_map" "otel_collector_config" {
  metadata {
    name      = "otel-collector-config"
    namespace = kubernetes_namespace.observability.metadata[0].name
  }

  data = {
    "config.yaml" = file("${path.module}/../app/observability/otel-collector-config.yaml")
  }
}

# Prometheus ConfigMap
resource "kubernetes_config_map" "prometheus_config" {
  metadata {
    name      = "prometheus-config"
    namespace = kubernetes_namespace.observability.metadata[0].name
  }

  data = {
    "prometheus.yml" = templatefile("${path.module}/configs/prometheus.yml", {
      app_namespace = kubernetes_namespace.go_ai_poi.metadata[0].name
    })
  }
}

# Loki ConfigMap
resource "kubernetes_config_map" "loki_config" {
  metadata {
    name      = "loki-config"
    namespace = kubernetes_namespace.observability.metadata[0].name
  }

  data = {
    "loki.yaml" = file("${path.module}/../app/observability/loki-config.yaml")
  }
}

# Tempo ConfigMap
resource "kubernetes_config_map" "tempo_config" {
  metadata {
    name      = "tempo-config"
    namespace = kubernetes_namespace.observability.metadata[0].name
  }

  data = {
    "tempo.yaml" = file("${path.module}/../app/observability/tempo-config.yaml")
  }
}

# Grafana Datasources ConfigMap
resource "kubernetes_config_map" "grafana_datasources" {
  metadata {
    name      = "grafana-datasources"
    namespace = kubernetes_namespace.observability.metadata[0].name
  }

  data = {
    "datasources.yaml" = file("${path.module}/../app/observability/grafana-datasources.yaml")
  }
}