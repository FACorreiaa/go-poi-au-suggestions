# Loki PersistentVolumeClaim
resource "kubernetes_persistent_volume_claim" "loki_pvc" {
  metadata {
    name      = "loki-pvc"
    namespace = kubernetes_namespace.observability.metadata[0].name
  }
  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = {
        storage = "5Gi"
      }
    }
  }
}

# Loki Deployment
resource "kubernetes_deployment" "loki" {
  metadata {
    name      = "loki"
    namespace = kubernetes_namespace.observability.metadata[0].name
    labels = {
      app = "loki"
    }
  }

  spec {
    replicas = 1
    selector {
      match_labels = {
        app = "loki"
      }
    }
    template {
      metadata {
        labels = {
          app = "loki"
        }
      }
      spec {
        container {
          image = "grafana/loki:latest"
          name  = "loki"
          
          args = ["-config.file=/etc/loki/loki.yaml"]

          port {
            container_port = 3100
            name          = "http-metrics"
          }

          port {
            container_port = 9096
            name          = "grpc"
          }

          volume_mount {
            name       = "loki-config"
            mount_path = "/etc/loki"
          }

          volume_mount {
            name       = "loki-storage"
            mount_path = "/var/loki"
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
              path = "/ready"
              port = 3100
            }
            initial_delay_seconds = 45
          }

          readiness_probe {
            http_get {
              path = "/ready"
              port = 3100
            }
            initial_delay_seconds = 45
          }
        }
        
        volume {
          name = "loki-config"
          config_map {
            name = kubernetes_config_map.loki_config.metadata[0].name
          }
        }

        volume {
          name = "loki-storage"
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.loki_pvc.metadata[0].name
          }
        }
      }
    }
  }
}

# Loki Service
resource "kubernetes_service" "loki" {
  metadata {
    name      = "loki-service"
    namespace = kubernetes_namespace.observability.metadata[0].name
  }
  spec {
    selector = {
      app = "loki"
    }
    port {
      port        = 3100
      target_port = 3100
      name        = "http-metrics"
    }
    port {
      port        = 9096
      target_port = 9096
      name        = "grpc"
    }
    type = "ClusterIP"
  }
}