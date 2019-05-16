job "fail-service" {
  datacenters = ["testing"]

  type = "service"

  # https://www.nomadproject.io/docs/job-specification/reschedule.html
  reschedule {
    delay          = "30s"
    delay_function = "constant"
    unlimited      = true
  }

  # https://www.nomadproject.io/docs/job-specification/update.html
  update {
    max_parallel      = 1
    health_check      = "checks"
    min_healthy_time  = "10s"
    healthy_deadline  = "5m"
    auto_revert       = true
    canary            = 0
    stagger           = "30s"
  }

  group "fail-service" {

    # https://www.nomadproject.io/docs/job-specification/restart.html
    restart {
      interval = "5m"
      attempts = 10
      delay    = "25s"
      mode     = "delay"
    }

    task "fail-service" {
      driver = "docker"
      config {
        image = "10.61.128.153:5000/service/fail_service:latest"
        port_map = {
          http = 8080
        }
      }

      # Register at consul
      service {
        name = "${TASK}"
        port = "http"
        check {
          port     = "http"
          type     = "http"
          path     = "/health"
          method   = "GET"
          interval = "10s"
          timeout  = "2s"
        }

        # https://www.nomadproject.io/docs/job-specification/check_restart.html
        check_restart {
          limit = 3
          grace = "10s"
          ignore_warnings = false
        }
      }

      env {
        HEALTHY_IN    = 0,
        HEALTHY_FOR   = -1,
        UNHEALTHY_FOR = 0,
        OOM_AFTER = 0,
      }

      resources {
        cpu    = 100 # MHz
        memory = 256 # MB
        network {
          mbits = 10
          port "http" {}
        }
      }
    }
  }
}