# Revisión de los textos de impacto

Revisión de la [issue #101](https://github.com/R0SEWT/aire-distribuido/issues/101):
textos de Impacto y de «¿De dónde sale este número?», contrastados con el informe
y sin atribuir a los datos lo que excluye la spec del TB2 en §7.2.

## Alcance y fuentes del cotejo

- [Spec del frontend](specs/2026-11-21-frontend-tb2-design.md): §5.7, §7.1–§7.4 y §13.4.
- Informe de PC3: [pregunta de impacto](informe/secciones/_02-problema.qmd) y
  [conclusiones 5, 7 y 8](informe/secciones/_09-conclusiones.qmd): el régimen no
  sustituye a la alerta, la cobertura es desigual y las campañas tienen límites.
- Informe de PC4: [alerta y clasificación](informe/secciones/pc4/_13-api.qmd) y
  [conclusiones 6 y 9](informe/secciones/pc4/_09-conclusiones.qmd): la alerta sale
  de la media de 24 h; la anticipación quedaba para el siguiente entregable.
- [Resultados de impacto](../reports/impacto.json), su copia en
  [la web](../web/public/datos/impacto.json) y [el generador](../scripts/impacto.py):
  reglas, denominadores, fuentes con SHA-256, señales y agrupación de episodios.

Los informes de PC3 y PC4 describen sus propias entregas. Los resultados nuevos
de anticipación se cotejan con `impacto.json`; no se atribuyen a esos informes
como si ya hubieran sido publicados allí. La redacción final del informe TB2
corresponde a la issue #103.

## Correcciones editoriales

| Tema | Problema de la redacción anterior | Redacción y alcance revisados |
|---|---|---|
| Datos históricos | El alcance temporal quedaba principalmente en el pie general. | La entrada de Impacto indica que son mediciones históricas y que no miden exposición de la población. |
| Alerta | «Es la norma misma […] por eso no falla» confundía aplicar una regla con garantizar datos o resultados infalibles. | Se compara la media disponible con los valores del ECA. No es una predicción ni una garantía sobre la calidad de las mediciones. Sin el mínimo de lecturas no hay media; eso no equivale a estar bajo el ECA. |
| Totales | «Horas sobre el ECA» podía leerse como la duración de un episodio o de todo el territorio. | Se llaman horas-estación: la misma hora en dos estaciones cuenta dos veces. El porcentaje usa como denominador solo las horas-estación con media calculable. |
| Beneficio y población | «Un aviso que hoy no tienen» afirmaba una situación previa no medida. | Se presenta el posible uso de la información. El censo es contexto por distrito y año, sin total de personas ni estimación de expuestos, usuarios atendidos o beneficio sanitario. |
| Cobertura | «Nunca superó» podía extenderse a periodos sin mediciones. | Se habla de superaciones registradas en el periodo analizado. Sin datos no se puede afirmar que el aire esté limpio. |
| Precisión y cobertura de las señales | Los porcentajes podían confundirse con una validación predictiva o con población cubierta. | Se explican sus distintos denominadores y que describen coincidencias sobre el conjunto histórico, no aciertos con datos nuevos ni cobertura territorial. |
| Episodios y anticipación | Un episodio se resumía como cada cruce del ECA, pero el generador agrupa las superaciones separadas por menos de una ventana. | Se explica cuándo empieza un episodio y se conservan juntas la primera señal y las horas seguidas. La anticipación es retrospectiva, con las limitaciones del archivo a la vista. |

## «¿De dónde sale este número?»

Cada explicación conserva sus reglas, rutas, filas y huellas. Se añadieron nombres
legibles a las fuentes: lecturas usadas, concentraciones sin estandarizar, régimen
de cada lectura, perfil del modelo, comprobación de alerta en Go, ficha de la
estación y población censada. La sección de señales enlaza también la fuente de
la comprobación de la alerta.

El enlace al informe se identifica como el contexto de PC3 y PC4. Las cifras
nuevas se atribuyen al archivo de impacto y a su generador, sin prometer que un
archivo de datos esté publicado cuando no se versiona.

## Verificación

Las expectativas de texto de `web/src/pantallas/Impacto.test.tsx` y la entrada de
Impacto en `web/src/App.test.tsx` se actualizaron
sin eliminar las comprobaciones de cifras dinámicas, fuentes, censo por fila,
ausencia de sumas de población, anticipación y limitaciones. No se añadieron
pruebas que solo repitan las frases nuevas.

Desde `web/`, con pnpm 10.20.0 y las dependencias del lockfile:

```bash
pnpm install --frozen-lockfile
pnpm lint
pnpm test
pnpm build
```

No se modificaron los JSON, sus huellas, las fórmulas, la población, el modelo,
la API ni los informes históricos. Esta revisión no acredita una exposición,
un impacto sanitario ni una evaluación con lecturas futuras.

## Capturas de la revisión

La interfaz se revisó en Chrome contra una instancia separada de la API real, con
MongoDB, Redis y tres nodos. Estas imágenes son capturas del navegador; no se
reconstruyeron los gráficos ni se editaron las cifras:

- [Alcance histórico y horas-estación](informe/capturas/tb2/textos-impacto/impacto-01-alcance.jpg).
- [Regla, denominador y archivos de origen](informe/capturas/tb2/textos-impacto/impacto-02-fuentes.jpg).
- [Precisión y cobertura de las señales](informe/capturas/tb2/textos-impacto/impacto-03-senales.jpg).
- [Limitaciones de los datos y la anticipación](informe/capturas/tb2/textos-impacto/impacto-04-limites.jpg).

La [procedencia de las capturas](informe/capturas/tb2/textos-impacto/procedencia.json)
identifica la base del código, los textos locales aún sin commit, las huellas del
código revisado y del JSON de impacto, y las de las propias imágenes. No son
evidencias de una corrida en el laboratorio ni de rendimiento en producción.
