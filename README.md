# godev 🛠️

[![Go Version](https://img.shields.io/badge/go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Release](https://img.shields.io/github/v/release/jmoyonero/godev?color=brightgreen)](https://github.com/jmoyonero/godev/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![CI](https://github.com/jmoyonero/godev/actions/workflows/ci.yml/badge.svg)](https://github.com/jmoyonero/godev/actions/workflows/ci.yml)

> **Herramienta CLI unificada para desarrollo en Go y microservicios.**  
> Estandariza la calidad de código, análisis de seguridad SAST, comprobación de vulnerabilidades (CVEs), tests unitarios con detección de condiciones de carrera, gestión de contenedores Docker y orquestación E2E completa con Robot Framework.

---

## 🎯 ¿Por qué `godev`?

Cuando tienes múltiples microservicios en Go, copiar y mantener `Makefiles` o scripts de bash idénticos genera:
- **Duplicación masiva:** Cambiar la versión de un linter o una regla de seguridad requiere editar 10 repositorios.
- **Inconsistencias entre desarrolladores:** Comandos que funcionan en Linux o CI pero fallan en macOS con diferencias en `lsof`, `kill` o rutas de Python.
- **Falta de estándares:** Cada microservicio termina teniendo flags y comandos diferentes.

`godev` centraliza todo en un **único binario nativo en Go**, rápido, tipado y sin dependencias externas obligatorias. En tus microservicios ya no necesitas ningún `Makefile`.

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

---

## 📋 Comandos Disponibles

### 1. Calidad, Linters y Seguridad

| Comando | Descripción |
| :--- | :--- |
| `godev verify` | **Pipeline completo:** ejecuta en secuencia `lint` + `sec` + `vulncheck` + `test` y emite un informe consolidado con tiempos. |
| `godev lint [--fix]` | Ejecuta `golangci-lint` con la versión fijada centralmente (soporta `--fix` para correcciones automáticas). |
| `godev sec` | Ejecuta análisis estático de seguridad SAST con `gosec` (excluyendo automáticamente código generado o mocks). |
| `godev vulncheck` | Escanea vulnerabilidades conocidas (CVEs) en las dependencias del proyecto con `govulncheck`. |
| `godev test [--race] [--shuffle] [--format] [--plain]` | Ejecuta tests unitarios en Go con flags configurables (`-race`, `-shuffle=on`). Si [`gotestsum`](https://github.com/gotestyourself/gotestsum) está instalado (`go install gotest.tools/gotestsum@latest`) formatea la salida con colores y resumen de fallos; `--format` (o `test.format`) elige el formato (`testname` por defecto, `pkgname`, `dots`, `testdox`, `pkgname-and-test-fails`...). Sin gotestsum, o con `--plain`, usa `go test -v`. |

### 2. Infraestructura Local (Docker Compose)

| Comando | Descripción |
| :--- | :--- |
| `godev infra up [servicios...]` | Levanta los contenedores en segundo plano y espera activamente a que los servicios estén `ready` (`pg_isready`). |
| `godev infra down [-v]` | Detiene los contenedores (con `-v` para eliminar volúmenes y reiniciar estado efímero). |
| `godev infra reset-db` | Aplica el script de datos semilla (`seeds.sql`) en la base de datos limpia. |
| `godev infra ps` | Muestra el estado actual de los contenedores del proyecto. |

### 3. Desarrollo y Ejecución de Servicios (`godev run`)

| Comando | Descripción |
| :--- | :--- |
| `godev run [servicios...]`<br>*(alias: `start`, `dev`)* | **Ejecutor concurrente en desarrollo:**<br>1. Compila concurrentemente los servicios declarados en `e2e.services` de `.godev.yaml` (o los indicados por argumento, ej: `godev run api`).<br>2. Libera puertos ocupados automáticamente.<br>3. Inyecta variables de entorno combinadas (`e2e.env` + `svc.env`).<br>4. Canaliza los logs de cada servicio con prefijos coloreados y alineados.<br>5. Espera activamente a que los healthchecks respondan OK.<br>6. Detiene limpiamente los procesos al pulsar `Ctrl+C`. |

Opciones:
```bash
godev run             # Compila y arranca todos los servicios
godev run api         # Arranca únicamente el servicio 'api'
godev run --reset-db  # Aplica seeds.sql en la base de datos antes de arrancar
```

### 4. End-to-End con Robot Framework

| Comando | Descripción |
| :--- | :--- |
| `godev e2e` (o `godev robot`) | **Orquestador inteligente E2E:**<br>1. Restaura base de datos con seeds.<br>2. Crea y configura el virtualenv de Python (`.venv`) e instala `requirements.txt` si no existe.<br>3. Libera puertos en uso.<br>4. Compila y arranca en background los servicios necesarios con sus variables de entorno.<br>5. Espera con healthcheck polling activo.<br>6. Ejecuta Robot Framework.<br>7. Abre automáticamente el reporte HTML en Google Chrome.<br>8. Garantiza el apagado y limpieza de procesos y binarios al terminar o al recibir Ctrl+C. |

Opciones adicionales:
```bash
godev e2e --no-browser    # No abre el reporte en el navegador
godev e2e --stop-infra    # Destruye los contenedores (-v) al terminar
godev e2e --suite ruta/   # Ejecuta una suite específica
```

### 5. Configuración y Utilidades

| Comando | Descripción |
| :--- | :--- |
| `godev init [nombre]` | Genera una plantilla de configuración `.godev.yaml` en el directorio actual. |
| `godev version` | Muestra la versión actual instalada de `godev`. |

---

## ⚙️ Configuración (`.godev.yaml`)

`godev` funciona sin configuración previa aplicando defaults inteligentes para Go. Si un microservicio necesita personalizar rutas, puertos o servicios en background, solo requiere un archivo `.godev.yaml`:

```yaml
name: loaney-api

infra:
  compose_file: deployments/docker-compose.yaml
  seeds_file: deployments/seeds.sql
  db_service: db
  db_user: admin
  db_name: loaney_db

lint:
  version: "v2.13.2"

sec:
  exclude_dirs:
    - "internal/oas"
    - "internal/mocks"

test:
  path: "./internal/..."
  race: true
  shuffle: "on"
  format: "testname"   # formato de gotestsum, si está instalado

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
    CLOUDSQL_PASSWORD: "secret_password"
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

---

## 👨‍💻 Autor

Creado y mantenido por **Jonathan Moyonero** ([@jmoyonero](https://github.com/jmoyonero)).

## 📄 Licencia

Distribuido bajo la Licencia MIT. Consulta [LICENSE](LICENSE) para más información.
