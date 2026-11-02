# aire-distribuido — AI Agent Instructions

Trabajo Final (TB2) del curso **Programación Concurrente y Distribuida (1ACC0065)**, UPC, 2026-20.
Lo trabajan Rody Vilchez, Dayana Gómez y Julio Meza. El cuaderno del curso (apuntes, labs, beads de
las evaluaciones, enunciado) vive en `../concurrente`; este repo es el sistema y su laboratorio.

## Contexto

- **Problema**: caso 10 del enunciado, Monitoreo Calidad del Aire y Sensores Ambientales: clustering
  de lecturas horarias de PM2.5/PM10 y meteorología para alertas tempranas de salud respiratoria.
  El equipo empezó con el caso 2 (denuncias SIDPOL) y el profesor lo reasignó al 10 el 31/10; el repo
  se llamaba `infelix-distribuido` (y la base de beads conserva ese prefijo).
- **Pregunta de impacto social**: no se fija a ciegas. Se elige el martes 03/11 con la criba del EDA
  entre las nueve candidatas de `docs/specs/2026-10-31-preguntas-candidatas.md`. La base
  esperada: episodios sobre el ECA, dónde, cuándo y con cuánta anticipación; el diferencial: lo que solo
  OEFA permite (sus estaciones están alrededor de actividades fiscalizadas).
- **Cobertura de estaciones**: 34 estaciones no cubren el país. Una zona sin estación no es una zona
  con aire limpio; cualquier mapa de "zonas de riesgo" tiene que decirlo. Es el ángulo ético del DD.
- **Datos** (`data/README.md`): OEFA, *Vigilancia y Seguimiento ambiental en la calidad del aire*,
  CSV de 1.3 GB en **formato largo** (una fila por estación, parámetro, fecha-hora, valor y periodo), es
  la fuente y la carga de >1M filas; SENAMHI Lima (577,794 filas) queda como contraste en el TB2. Nada
  se versiona salvo `data/sample/` (muestra **sintética** mientras no llegue el CSV real; el portal
  bloquea la descarga por terminal, se baja desde el navegador o Colab).
- **Algoritmo**: K-Means (Lloyd) paralelo, portado del K-Means del TP. Cada chunk o nodo devuelve sumas
  y conteos parciales por cluster; la fusión en orden fijo es exacta; el coordinador recalcula los
  centroides y repite. La receta (features, log1p, nivel, k, semilla) se calibra en Colab y Go la lee
  de `configs/kmeans.json`: Go no decide features ni k.
- **Specs de diseño** (se escriben y revisan antes de codificar): `docs/specs/`. La de la
  PC3, la de calibración del K-Means y la de preguntas candidatas. `docs/propuesta.md` es el borrador
  anterior, superado por las specs.

## Arquitectura

Las cuatro partes del enunciado, una por entregable:

| Parte | Qué hace | Entregable |
|------|---------|-----------|
| A — Cargador concurrente (`datos/`, `cmd/cargar`) | bloques por bytes → workers (reglas, pivot por bloque) → recolector en orden de bloque → z-score con Welford y Chan; escribe silver, gold y reports | PC3 (07/11) |
| B — K-Means paralelo (`kmeans/`, `cmd/entrenar`, `cmd/benchmark`) | worker pool por chunks sobre `Parciales`/`Fusionar`/`Actualizar`; regímenes con etiqueta ECA, dónde y cuándo, codo | PC3; distribuido en la PC4 (`protocolo/`, `docs/diseno.md`) |
| C — API coordinadora | REST/WebSocket en Go, <100 ms, MongoDB + Redis | PC4 (21/11) |
| D — Frontend SPA | mapa de estaciones con su régimen y las alertas en tiempo real | TB2 (29/11) |

**Capas medallón en Google Drive** (`aire-distribuido/data/{bronze,silver,gold,reports}`, remoto
`rclone` `gdrive:`, nunca `cipdrive:`): bronze = CSV largo tal cual; silver y gold **los produce Go**;
`reports/` guarda las salidas chicas versionadas. **Colab es el laboratorio** (`notebooks/`: criba y
EDA, calibración, auditoría y figuras); **Go es el sistema**. Python nunca es parte del sistema.

```bash
GO=go   # si `go` falla por snap-confine en un sandbox: GO=/snap/go/current/bin/go
$GO test -race ./...                                   # siempre con -race; el CI corre -short
$GO run ./cmd/cargar -entrada data/sample/oefa_muestra.csv -workers 4      # silver, gold, reports
$GO run ./cmd/entrenar -codo 2:10                      # regímenes, ECA, codo (lee configs/kmeans.json)
$GO run ./cmd/benchmark -workers 1,2,4,8,11 -chunk 1000,10000 -repes 5     # speedup del K-Means
docker build -t aire-distribuido . && docker run --rm aire-distribuido cargar -h
cd docs/informe && bash render.sh                      # Quarto → los tres .docx (salida/)
```

## Archivos clave

| Archivo | Propósito |
|------|---------|
| `docs/specs/2026-10-31-pc3-design.md` | Spec de la PC3: datos, reglas, cargador, K-Means, diseño distribuido, informe, calendario, decisiones. |
| `docs/specs/2026-10-31-kmeans-calibracion-design.md` | Qué tiene que decir un régimen y la grilla que congela `configs/kmeans.json`. |
| `docs/specs/2026-10-31-preguntas-candidatas.md` | Las nueve preguntas y la criba del EDA que decide cuál sobrevive. |
| `configs/limpieza.json` | Columnas, parámetros, periodos, unidades, centinelas y rangos del CSV. **Supuestos hasta ver el CSV real**; `reports/limpieza.json → vistos` dice qué llegó. |
| `configs/kmeans.json` | Receta congelada por la calibración: features, log1p, nivel, k, semilla, tolerancia, chunk. |
| `datos/doc.go` | Pipeline del cargador, cascada de reglas y la config campo por campo. |
| `kmeans/doc.go` | Contrato numérico heredado del TP y las interfaces `Parciales`/`Fusionar`/`Actualizar`. |
| `docs/diseno.md`, `docs/diagramas/` | Diseño del K-Means paralelo y distribuido, patrones de Go, protocolo; diagramas de arquitectura y de una iteración. |
| `protocolo/` | Mensajes JSON por línea sobre TCP del clúster (PC4) con sus tests: fusión en orden de nodo, timeout, rezagados. |
| `docs/informe/` | Informe en Quarto → Word: `render.sh`, `tablas.py` (cifras desde `generado/*.json`, nunca a mano), `empaquetar-pc3.sh`. |
| `notebooks/` | 01 criba y EDA, 02 calibración, 03 auditoría y figuras. Corren en Colab sobre Drive. |
| `data/README.md`, `data/sample/README.md` | Fuentes y descarga; qué supone la muestra sintética. |
| `.github/workflows/go.yml` | CI: `go test -race -short`, `go vet`, `gofmt`. |
| `../concurrente/materials/extra/CC65-TB2-1ACC0065_202620.pdf` | Enunciado y rúbrica (no versionado: es del profesor). |

## Convenciones

- **Spec antes que código**: cada parte tiene su diseño en `docs/specs/` revisado contra el
  enunciado y la rúbrica antes de implementar. Las decisiones abiertas se marcan en la spec.
- **Solo biblioteca estándar de Go**; configs en JSON por eso. TDD y `go test -race` siempre.
- **Determinismo**: mismo resultado bit a bit para cualquier número de workers (reducciones en orden
  de bloque o de chunk, filas ordenadas por estación y fecha-hora); con tolerancia entre nodos y
  contra el secuencial.
- **Contratos entre partes** (no se cambian sin avisar a las dos): silver `estacion,fecha_hora,` +
  parámetros canónicos; gold `estacion,fecha_hora,` + features de `configs/kmeans.json`, estandarizadas,
  solo filas completas, RFC3339 con `-05:00`, orden por estación y fecha-hora; `reports/estandarizacion.json`
  con `n` igual a las filas del gold (`entrenar` falla si no).
- **Git Flow**: `develop` es la rama por defecto; trabajo en `feature/*` → PR a `develop`; cada
  entrega sale de `release/pc3`, `release/pc4`, `release/tb2` → PR a `main`. Nunca abrir una PR con
  head=`develop`.
- **Issues de GitHub solo para problemas**. El trabajo se sigue en beads (epic `infelix-distribuido-5ww`
  para la PC3). El tablero Kanban se crea el lunes de cada entrega y se mueve durante la semana.
- **Informe en Word generado con Quarto**, nombre `1ACC0065_PC3_202610_Informe_UXXXXXXXX` (el
  enunciado dice 202610). Ninguna cifra a mano: todo sale de `reports/` vía `tablas.py`. Capturas de
  terminal reales. El uso de IA se declara en un anexo.
- **Commits, PRs e issues en español**, sin metadata de IA.
- Equipo: si Dayana y Julio se suben, se reparte; si no, el trabajo avanza igual.

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:6cd5cc61 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/SYNC_CONCEPTS.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->
