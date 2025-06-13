# Install NGINX Ingress Controller using Helm
resource "helm_release" "nginx_ingress" {
  count      = var.enable_ingress ? 1 : 0
  name       = "nginx-ingress"
  repository = "https://kubernetes.github.io/ingress-nginx"
  chart      = "ingress-nginx"
  namespace  = "ingress-nginx"
  create_namespace = true

  set {
    name  = "controller.replicaCount"
    value = "2"
  }

  set {
    name  = "controller.nodeSelector.kubernetes\\.io/os"
    value = "linux"
  }

  set {
    name  = "controller.service.type"
    value = "LoadBalancer"
  }

  set {
    name  = "controller.admissionWebhooks.enabled"
    value = "false"
  }
}

# Application Ingress
resource "kubernetes_ingress_v1" "app_ingress" {
  count = var.enable_ingress ? 1 : 0
  metadata {
    name      = "loci-server-ingress"
    namespace = kubernetes_namespace.go_ai_poi.metadata[0].name
    annotations = {
      "nginx.ingress.kubernetes.io/rewrite-target" = "/"
      "nginx.ingress.kubernetes.io/ssl-redirect"   = "false"
    }
  }

  spec {
    ingress_class_name = "nginx"
    rule {
      host = var.domain
      http {
        path {
          backend {
            service {
              name = kubernetes_service.go_ai_poi_app.metadata[0].name
              port {
                number = 8000
              }
            }
          }
          path      = "/"
          path_type = "Prefix"
        }
      }
    }
  }

  depends_on = [helm_release.nginx_ingress]
}

# Observability Ingress
resource "kubernetes_ingress_v1" "observability_ingress" {
  count = var.enable_ingress ? 1 : 0
  metadata {
    name      = "observability-ingress"
    namespace = kubernetes_namespace.observability.metadata[0].name
    annotations = {
      "nginx.ingress.kubernetes.io/rewrite-target" = "/"
      "nginx.ingress.kubernetes.io/ssl-redirect"   = "false"
    }
  }

  spec {
    ingress_class_name = "nginx"
    
    # Grafana
    rule {
      host = "grafana.${var.domain}"
      http {
        path {
          backend {
            service {
              name = kubernetes_service.grafana.metadata[0].name
              port {
                number = 3000
              }
            }
          }
          path      = "/"
          path_type = "Prefix"
        }
      }
    }

    # Prometheus
    rule {
      host = "prometheus.${var.domain}"
      http {
        path {
          backend {
            service {
              name = kubernetes_service.prometheus.metadata[0].name
              port {
                number = 9090
              }
            }
          }
          path      = "/"
          path_type = "Prefix"
        }
      }
    }
  }

  depends_on = [helm_release.nginx_ingress]
}