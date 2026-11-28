<!-- Generado por tablas.py desde reports/impacto.json. No editar a mano. -->

::: {#tbl-impacto-estaciones tbl-colwidths="[13,23,16,15,9,11,13]"}
| Estación | Actividad y objeto fiscalizado | Distrito y población (censo 2017) | Periodo con datos | Horas con media | Sobre el ECA | Banda más común |
|:---|:---|:---|---:|---:|---:|:---|
| CA-CP-01 | corredor vial: Corredor vial Apurímac – Cusco | Capacmarca (Cusco), 3 866 | 2020-01-23 a 2021-12-31 (704 días) | 16 492 | 321 (2 %) | buena |
| CA-HU-04 | ladrilleras: Zona ladrillera de centro poblado menor Santa María de Huachipa y Nievería | Lurigancho (Lima), 240 814 | 2021-12-10 a 2021-12-20 (11 días) | 237 | 237 (100 %) | mala |
| CA-RZC-02 | metalurgia (refinación o fundición): Refinería de zinc Cajamarquilla de Nexa Resources Cajamarquilla S.A.C \| Refinería de zinc Cajamarquilla | Lurigancho (Lima), 240 814 | 2021-12-19 a 2021-12-29 (11 días) | 228 | 215 (94 %) | mala |
| CA-ESP-01 | corredor vial: Corredor vial Apurímac – Cusco, distrito y provincia de Espinar | Espinar (Cusco), 34 861 | 2021-11-15 a 2021-12-30 (46 días) | 1 070 | 192 (18 %) | buena |
| CA-HU-09 | ladrilleras: Zona ladrillera de centro poblado menor Santa María de Huachipa y Nievería | Lurigancho (Lima), 240 814 | 2021-12-14 a 2021-12-31 (18 días) | 414 | 133 (32 %) | mala |
| CA-AYAS-01 | minería: Unidad minera Antamina | San Marcos (Ancash), 17 033 | 2021-12-10 a 2022-12-31 (318 días) | 7 188 | 58 (1 %) | buena |
| CA-CG-02 | agroindustria: Empresa Casa Grande S.A.A | Casa Grande (La Libertad), 28 492 | 2022-02-14 a 2022-12-31 (264 días) | 6 001 | 25 (0 %) | moderada |
| CA-SOC-01 | (no lo dice el nombre): Unidad de producción Cerro Verde S.A.A | Socabaya (Arequipa), 75 351 | 2021-12-03 a 2021-12-31 (29 días) | 679 | 3 (0 %) | mala |
| CA-CG-01 | agroindustria: Empresa Casa Grande S.A.A \| Empresa agroindustrial Casa Grande S.A.A | Casa Grande (La Libertad), 28 492 | 2021-12-03 a 2022-02-13 (46 días) | 1 027 | 0 (0 %) | moderada |
| CA-CH-03 | zona industrial: Zona industrial Gran Trapecio | Chimbote (Ancash), 206 213 | 2021-11-11 a 2021-12-31 (51 días) | 1 184 | 0 (0 %) | moderada |
| CA-COLQ-01 | minería: Unidad Minera Colquijirca | Tinyahuarco (Pasco), 6 755 | 2021-12-03 a 2021-12-31 (26 días) | 518 | 0 (0 %) | buena |
| CA-HUA-01 | (no lo dice el nombre): Empresa Agroaurora S.A.C | La Huaca (Piura), 12 950 | 2021-12-08 a 2022-12-31 (315 días) | 7 380 | 0 (0 %) | buena |
| CA-HUAR-01 | minería: Unidad Minera Colquijirca | (la ficha no trae distrito) | 2021-12-05 a 2021-12-16 (12 días) | 265 | 0 (0 %) | buena |
| CA-JUPR-01 | minería: Unidad minera Antamina | San Marcos (Ancash), 17 033 | 2022-05-01 a 2022-12-31 (245 días) | 5 727 | 0 (0 %) | buena |
| CA-MAC-01 | (no lo dice el nombre): Empresa Agroaurora S.A.C | La Huaca (Piura), 12 950 | 2021-12-09 a 2022-12-31 (349 días) | 8 148 | 0 (0 %) | buena |
| CA-ROM-01 | agroindustria: Empresa Casa Grande S.A.A \| Empresa agroindustrial Casa Grande S.A.A | Casa Grande (La Libertad), 28 492 | 2021-12-03 a 2022-12-31 (341 días) | 7 913 | 0 (0 %) | moderada |
| CA-TAL-01 | (no lo dice el nombre): Unidad fiscalizable Quellaveco | Torata (Moquegua), 6 198 | 2021-12-07 a 2021-12-31 (14 días) | 277 | 0 (0 %) | buena |
| **Total** |  | 10 distritos |  | **64 748** | **1 184 (1,8 %)** |  |

Horas con la media de 24 horas sobre el ECA, por estación, con su actividad fiscalizada y la población de su distrito
:::

*Nota.* Fuente: `reports/impacto.json`, escrito por `scripts/impacto.py` el 2026-11-21 sobre el gold y el silver (sha256 en el archivo) y `reports/poblacion_distritos.csv`. Una hora cuenta si su media de 24 horas es calculable (18 lecturas o más en la ventana, la regla de `ventana/ventana.go`). «Sobre el ECA»: PM2,5 > 50 o PM10 > 100 µg/m³ en esa media. La población es la censada del distrito (INEI, Censos Nacionales 2017; la página de cada cifra está en el archivo) y es contexto de la fila: no se suma, porque vivir en el distrito no es respirar el aire de la estación, que mide junto a la actividad fiscalizada.
