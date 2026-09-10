# godev 🛠️

Herramienta CLI unificada para desarrollo en Go y microservicios.

Estandariza linters, análisis de seguridad SAST, comprobación de vulnerabilidades (CVEs), tests unitarios con detección de race conditions, gestión de infraestructura Docker y orquestación E2E con Robot Framework.

## 🚀 Instalación

```bash
go install github.com/jmoyonero/godev@latest
```

## 📋 Comandos Disponibles

### Calidad y Seguridad
- `godev lint`: Ejecuta `golangci-lint` (soporta `--fix` para correcciones automáticas).
- `godev sec`: Ejecuta análisis de seguridad SAST con `gosec`.
- `godev vulncheck`: Escanea vulnerabilidades conocidas en dependencias con `govulncheck`.
- `godev test`: Ejecuta tests unitarios con `-race` y `-shuffle=on`.
- `godev verify`: Ejecuta el pipeline completo de calidad (`lint` + `sec` + `vulncheck` + `test`).

### Infraestructura Local
- `godev infra up`: Levanta los contenedores en segundo plano.
- `godev infra down [-v]`: Detiene los contenedores (y opcionalmente elimina volúmenes).
- `godev infra reset-db`: Restablece la base de datos limpia ejecutando `seeds.sql`.
- `godev infra ps`: Consulta el estado de los contenedores.

### End-to-End (Robot Framework)
- `godev e2e` (o `godev robot`):
  - Verifica o crea automáticamente el entorno virtual de Python.
  - Instala dependencias si faltan.
  - Limpia puertos en uso.
  - Compila y arranca servicios necesarios en segundo plano.
  - Espera activamente a los healthchecks.
  - Lanza Robot Framework.
  - Abre el reporte HTML en Google Chrome.
  - Limpia y apaga todos los procesos al finalizar o al cancelar (Ctrl+C).

### Configuración
- `godev init`: Genera una plantilla de configuración `.godev.yaml` en el proyecto actual.

## ⚙️ Configuración (`.godev.yaml`)

Si un microservicio necesita personalizar puertos, binarios o rutas:

```yaml
name: mi-servicio
compose_file: deployments/docker-compose.yaml
seeds_file: deployments/seeds.sql
db_service: db
db_user: admin
db_name: mi_db

lint:
  version: "v1.64.8"

sec:
  exclude_dirs:
    - "internal/oas"
    - "internal/mocks"

test:
  path: "./..."
  race: true
  shuffle: "on"

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
  services:
    - name: api
      cmd: ./cmd/api
      port: 8888
      health_url: http://127.0.0.1:8888/health
      env:
        PORT: "8888"
        LOG_LEVEL: "info"
```
