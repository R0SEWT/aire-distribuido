<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/limpieza.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/cargar` corra sobre el CSV completo.
:::

::: {#tbl-cobertura tbl-colwidths="[50,20,30]"}
| Departamento | Estaciones | Horas completas (gold) |
|:---|---:|---:|
| Arequipa | 1 | 166 |
| Junín | 1 | 131 |
| Lima | 1 | 165 |
| Ucayali | 1 | 155 |

Cobertura de la red por departamento
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `e1cef10`, 2026-11-01, máquina A019803). Un departamento que no aparece no tiene estación en el dataset: no hay dato, no aire limpio.
