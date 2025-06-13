# OpenTelemetry Collector Deployment
resource "kubernetes_deployment" "otel_collector" {
  metadata {
    name      = "otel-collector"
    namespace = kubernetes_namespace.observability.metadata[0].name
    labels = {
      app = "otel-collector"
    }
  }

  spec {
    replicas = 1
    selector {
      match_labels = {
        app = "otel-collector"
      }
    }
    template {
      metadata {
        labels = {
          app = "otel-collector"
        }
        annotations = {
          "prometheus.io/scrape" = "true"
          "prometheus.io/port"   = "8888"
        }
      }
      spec {
        container {
          image = "otel/opentelemetry-collector-contrib:latest"
          name  = "otel-collector"
          
          args = ["--config=/conf/config.yaml"]

          port {
            container_port = 4317
            name          = "otlp-grpc"
            protocol      = "TCP"
          }

          port {
            container_port = 4318
            name          = "otlp-http"
            protocol      = "TCP"
          }

          port {
            container_port = 8888
            name          = "metrics"
            protocol      = "TCP"
          }

          port {
            container_port = 8889
            name          = "prometheus"
            protocol      = "TCP"
          }

          volume_mount {
            name       = "otel-collector-config"
            mount_path = "/conf"
          }

          resources {
            limits = {
              memory = "1Gi"
              cpu    = "500m"
            }
            requests = {
              memory = "512Mi"
              cpu    = "250m"
            }
          }

          liveness_probe {
            http_get {
              path = "/"
              port = 13133
            }
            initial_delay_seconds = 45
          }

          readiness_probe {
            http_get {
              path = "/"
              port = 13133
            }
            initial_delay_seconds = 45
          }
        }
        
        volume {
          name = "otel-collector-config"
          config_map {
            name = kubernetes_config_map.otel_collector_config.metadata[0].name
          }
        }
      }
    }
  }
}

# OpenTelemetry Collector Service
resource "kubernetes_service" "otel_collector" {
  metadata {
    name      = "otel-collector-service"
    namespace = kubernetes_namespace.observability.metadata[0].name
    annotations = {
      "prometheus.io/scrape" = "true"
      "prometheus.io/port"   = "8888"
    }
  }
  spec {
    selector = {
      app = "otel-collector"
    }
    port {
      port        = 4317
      target_port = 4317
      name        = "otlp-grpc"
      protocol    = "TCP"
    }
    port {
      port        = 4318
      target_port = 4318
      name        = "otlp-http"
      protocol    = "TCP"
    }
    port {
      port        = 8888
      target_port = 8888
      name        = "metrics"
      protocol    = "TCP"
    }
    type = "ClusterIP"
  }
}