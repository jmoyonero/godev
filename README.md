# godev 🛠️

[![Go Version](https://img.shields.io/badge/go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Release](https://img.shields.io/github/v/release/jmoyonero/godev?color=brightgreen)](https://github.com/jmoyonero/godev/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![CI](https://github.com/jmoyonero/godev/actions/workflows/ci.yml/badge.svg)](https://github.com/jmoyonero/godev/actions/workflows/ci.yml)

> **Herramienta CLI unificada para desarrollo en Go y microservicios.**  
> Estandariza la calidad de código, análisis de seguridad SAST, comprobación de vulnerabilidades (CVEs), tests con detección de condiciones de carrera y cobertura, generación de código y mocks, imágenes Docker distroless, infraestructura local con Docker Compose y orquestación E2E completa con Robot Framework.

---

## 🎯 ¿Por qué `godev`?

Cuando tienes múltiples microservicios en Go, copiar y mantener `Makefiles` o scripts de bash idénticos genera:
- **Duplicación masiva:** Cambiar la versión de un linter o una regla de seguridad requiere editar 10 repositorios.
- **Inconsistencias entre desarrolladores:** Comandos que funcionan en Linux o CI pero fallan en macOS con diferencias en `lsof`, `kill` o rutas de Python.
- **Falta de estándares:** Cada microservicio termina teniendo flags y comandos diferentes.

`godev` centraliza todo en un **único binario nativo en Go**. En tus microservicios ya no necesitas ningún `Makefile`, ni un `Dockerfile`, ni un `docker-compose.yaml` propio.

---

## 🚀 Instalación

### Con `go install` (Recomendado)
```bash
go install github.com/jmoyonero/godev@latest
```

Asegúrate de tener `$GOPATH/bin` en tu `PATH`:
```bash
export PATH="$HOME/go/bin:$PATH"
```

### Requisitos

- **Go 1.26+.** `golangci-lint` (en la versión de `lint.version`), `gosec` y `govulncheck` se ejecutan con `go run`, sin instalarlos a mano. `godev generate` usa el `mockgen` de `go.uber.org/mock` declarado en el `go.mod` del microservicio.
- **Docker** con Compose v2, para `infra`, `run`, `e2e` y `build-image`.
- **Python 3**, solo para `e2e` (el virtualenv lo crea `godev`).
- **Opcional:** [`gotestsum`](https://github.com/gotestyourself/gotestsum) para una salida de tests más legible (`go install gotest.tools/gotestsum@latest`).

---

## 📋 Comandos Disponibles

### 1. Calidad, Linters y Seguridad

| Comando | Descripción |
| :--- | :--- |
| `godev verify [--skip-sec] [--skip-vuln]` | **Pipeline completo:** ejecuta en secuencia `lint` + `sec` + `vulncheck` + `test` y emite un informe consolidado con tiempos. |
| `godev lint [--fix]` | Ejecuta `golangci-lint` con la versión fijada en `lint.version` (por defecto la línea v2). Soporta `--fix` para correcciones automáticas. |
| `godev sec` | Análisis estático de seguridad SAST con `gosec`, excluyendo los directorios de `sec.exclude_dirs` (código generado y mocks por defecto). |
| `godev vulncheck` | Escanea vulnerabilidades conocidas (CVEs) en las dependencias con `govulncheck`. |
| `godev test` | Ejecuta los tests unitarios con `-race` y `-shuffle=on`. Usa `gotestsum` si está instalado y puede generar el informe de cobertura. |

Opciones de `godev test`:
```bash
godev test --path ./internal/...        # Paquetes a testear (por defecto ./...)
godev test --exclude-dir internal/it    # Excluye directorios o paquetes (repetible)
godev test --race=false --shuffle=off   # Desactiva el detector de carreras o el orden aleatorio
godev test --format testdox             # Formato de gotestsum (testname, pkgname, dots, testdox...)
godev test --plain                      # Usa 'go test -v' aunque gotestsum esté instalado
godev test --cover                      # Genera coverage.out y muestra el total al terminar
godev test --cover-profile cov.out      # Fichero del perfil de cobertura
godev test --html                       # Abre el informe HTML de cobertura (implica --cover)
```

### 2. Generación de Código y Mocks

| Comando | Descripción |
| :--- | :--- |
| `godev generate`<br>*(alias: `gen`, `mocks`)* | Ejecuta `go generate ./...` (p. ej. OpenAPI con ogen) y genera con `mockgen` los mocks de todas las interfaces del módulo en `internal/mocks`. |

```bash
godev generate --mocks-only   # Solo mocks, sin 'go generate'
godev generate --skip-mocks   # Solo 'go generate', sin mocks
```

### 3. Imágenes Docker

`godev` genera un **Dockerfile universal** multi-stage: detecta cada binario en `cmd/` y crea un target por cada uno. Los binarios se compilan estáticos y se ejecutan sobre `gcr.io/distroless/static-debian12:nonroot` (sin shell, usuario no root).

| Comando | Descripción |
| :--- | :--- |
| `godev dockerfile [-w]` | Imprime el Dockerfile universal, o lo escribe en `./Dockerfile` con `-w`. |
| `godev build-image`<br>*(alias: `docker-build`)* | Construye la imagen en local con el Dockerfile embebido, sin necesidad de tenerlo en el repo. |

```bash
godev build-image -t scheduler                   # Target a construir (por defecto api)
godev build-image -t api -i mi-api:1.2.0         # Tag de la imagen (por defecto <target>:latest)
godev build-image --ssh-key ~/.ssh/deploy_key    # Clave para módulos Go privados
godev build-image --no-cache                     # Construye sin caché
```

Para los módulos privados, la clave SSH se toma de `--ssh-key`, de `SSH_DEPLOY_KEY_B64` / `SSH_DEPLOY_KEY` o de `~/.ssh/id_ed25519` / `~/.ssh/id_rsa`. Se usa solo durante `go mod download` y no queda en la imagen.

### 4. Infraestructura Local (Docker Compose)

Si el repo no tiene `docker-compose.yaml`, `godev` genera uno al vuelo con los servicios de `infra.services`:

| Servicio | Imagen | Puerto por defecto |
| :--- | :--- | :--- |
| `db` | PostgreSQL 18 | `5432` |
| `wiremock` | WireMock 3 (mappings de `infra.wiremock_dir`) | `8090` |
| `jaeger` | Jaeger all-in-one | `16686` (UI) |
| `otel-collector` | OpenTelemetry Collector | `4317` (OTLP gRPC) |
| `prometheus` | Prometheus (activa `otel-collector`) | `9090` |
| `grafana` | Grafana con dashboards precargados (activa `prometheus`) | `3000` |

Por defecto se levantan `db`, `wiremock` y `jaeger`. Si el repo ya tiene un compose (`compose_file`, `test/infra/`, `infra/`, `deployments/` o la raíz), se usa ese.

> **Una sola infraestructura a la vez.** Todos los repos levantan su infra bajo el mismo proyecto de Compose (`godev`). Al levantar la de un repo se destruye la anterior (sea del proyecto que sea), volúmenes incluidos, así que nunca hay dos stacks peleando por los mismos puertos. Los datos son desechables: cada ejecución empieza con una base de datos limpia.

| Comando | Descripción |
| :--- | :--- |
| `godev infra up [servicios...]` | Destruye la infra previa, levanta la de este repo en segundo plano y espera a que los servicios estén listos (`pg_isready`, healthchecks HTTP). |
| `godev infra down [-v]` | Detiene los contenedores (`-v` / `--volumes` elimina también los volúmenes). |
| `godev infra reset-db` | Levanta la infra y aplica el script de datos semilla (`seeds.sql`). |
| `godev infra ps` | Muestra el estado de los contenedores. |

### 5. Desarrollo y Ejecución de Servicios (`godev run`)

| Comando | Descripción |
| :--- | :--- |
| `godev run [servicios...]`<br>*(alias: `start`, `dev`)* | **Ejecutor concurrente en desarrollo:**<br>1. Levanta la infraestructura (destruyendo la de cualquier otro proyecto).<br>2. Compila concurrentemente los servicios declarados en `e2e.services` de `.godev.yaml` (o los indicados por argumento, ej: `godev run api`).<br>3. Libera los puertos ocupados, parando el contenedor que los publique en lugar de matar procesos a ciegas.<br>4. Inyecta variables de entorno combinadas (`e2e.env` + `svc.env`).<br>5. Canaliza los logs de cada servicio con prefijos coloreados y alineados.<br>6. Espera activamente a que los healthchecks respondan OK.<br>7. Detiene limpiamente los procesos al pulsar `Ctrl+C`. |

Opciones:
```bash
godev run             # Compila y arranca todos los servicios
godev run api         # Arranca únicamente el servicio 'api'
godev run --reset-db  # Aplica seeds.sql en la base de datos antes de arrancar (-r)
```

### 6. End-to-End con Robot Framework

| Comando | Descripción |
| :--- | :--- |
| `godev e2e` (o `godev robot`) | **Orquestador inteligente E2E:**<br>1. Destruye la infraestructura local previa (sea del proyecto que sea) y levanta la de este.<br>2. Restaura base de datos con seeds si el repo los define.<br>3. Crea y configura el virtualenv de Python (`.venv`) e instala `requirements.txt` si no existe.<br>4. Libera puertos en uso.<br>5. Compila y arranca en background los servicios necesarios con sus variables de entorno.<br>6. Espera con healthcheck polling activo.<br>7. Ejecuta Robot Framework.<br>8. Abre automáticamente el reporte HTML en Google Chrome.<br>9. Destruye contenedores, procesos y binarios al terminar, pasen o fallen los tests, o al recibir Ctrl+C (salvo los contenedores con `--keep-infra`). |

Opciones adicionales:
```bash
godev e2e --no-browser    # No abre el reporte en el navegador
godev e2e --keep-infra    # No destruye la infraestructura al terminar (para repetir ejecuciones)
godev e2e --suite ruta/   # Ejecuta una suite específica
```

### 7. Configuración y Utilidades

| Comando | Descripción |
| :--- | :--- |
| `godev init [nombre]` | Genera una plantilla de configuración `.godev.yaml` en el directorio actual con todos los valores por defecto. |
| `godev version` | Muestra la versión actual instalada de `godev`. |

---

## ⚙️ Configuración (`.godev.yaml`)

`godev` funciona sin configuración previa aplicando defaults inteligentes para Go. Si un microservicio necesita personalizar rutas, puertos o servicios en background, solo requiere un archivo `.godev.yaml` (o `.godev.yml`). Todos los campos son opcionales:

```yaml
name: loaney-api

infra:
  # compose_file: deployments/docker-compose.yaml  # Sin él, se genera uno al vuelo
  services: [db, wiremock, jaeger]  # Añade prometheus/grafana para observabilidad
  seeds_file: test/seeds.sql
  wiremock_dir: test/wiremock
  db_service: db
  db_user: admin
  db_password: postgres
  db_name: loaney_db
  db_port: 5432
  wiremock_port: 8090
  prometheus_port: 9090
  grafana_port: 3000
  otel_port: 4317

lint:
  version: "v2.13.2"

sec:
  exclude_dirs:
    - "internal/oas"
    - "internal/mocks"

test:
  path: "./internal/..."
  exclude_dirs: []
  race: true
  shuffle: "on"
  format: testname           # Formato de gotestsum
  cover: false               # true = --cover por defecto
  cover_profile: coverage.out

e2e:
  enabled: true
  type: robot
  venv_dir: test/robot/.venv
  requirements: test/robot/requirements.txt
  suite_dir: test/robot
  results_dir: test/robot/results
  open_report: true
  variables:
    API_BASE_URL: "http://127.0.0.1:8888"
    SCHEDULER_BASE_URL: "http://127.0.0.1:8080"
  env:
    CLOUDSQL_CONNECTION_NAME: "127.0.0.1"
    CLOUDSQL_CONNECTION_PORT: "5432"
    CLOUDSQL_DB: "loaney_db"
    CLOUDSQL_USER: "admin"
    CLOUDSQL_PASSWORD: "postgres"
    LOANEY_API_PROVIDER_BASE_URL: "http://127.0.0.1:8090"
  services:
    - name: api
      cmd: ./cmd/api
      port: 8888
      health_url: "http://127.0.0.1:8888/health"
      env:
        PORT: "8888"
    - name: scheduler
      cmd: ./cmd/scheduler
      port: 8080
      health_url: "http://127.0.0.1:8080/healthz"
      env:
        PORT: "8080"
```

Las rutas de `seeds_file` y `wiremock_dir` se autodetectan si no se indican (`test/`, `test/infra/`, `infra/`, `deployments/` o la raíz).

---

## 👨‍💻 Autor

Creado y mantenido por **Jonathan Moyonero** ([@jmoyonero](https://github.com/jmoyonero)).

## 📄 Licencia

Distribuido bajo la Licencia MIT. Consulta [LICENSE](LICENSE) para más información.
