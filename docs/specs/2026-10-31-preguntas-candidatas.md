# Preguntas candidatas: qué problema puede abordar este dato

> **Borrador para la criba del EDA** (2026-10-31). El enunciado pide una pregunta de impacto social
> «documentada desde el inicio». No la tenemos, y no conviene inventarla antes de ver los datos: el
> primer objetivo del notebook 01 (EDA) es **medir las propiedades del dato que deciden qué pregunta
> sobrevive**. Esta ficha lista las candidatas, qué tiene que cumplir el dato para que cada una sea
> respondible, qué aporta el K-Means y a quién le importa fuera de la universidad. El martes 03/11
> queda una elegida y documentada; el informe cuenta que se eligió con el EDA, lo que es más honesto
> que una pregunta fijada a ciegas.

## 0. Lo que sabemos del dato antes de abrirlo

- OEFA no es SENAMHI: sus 34 estaciones están donde hay una **actividad fiscalizada** (fundiciones,
  minas, puertos de minerales, zonas industriales: La Oroya, Cerro de Pasco, Ilo, Callao, etc.). El
  dato no describe «el aire del Perú», describe **el aire alrededor de lo que OEFA vigila**. Cualquier
  pregunta que se elija hereda eso, y es también lo que lo hace distinto de un dataset urbano.
- Lecturas horarias en formato largo: parámetro, fecha, valor, periodo. Meteorología completa,
  material particulado y gases. Cobertura por estación y por parámetro: desconocida hasta el EDA.
- Rango de fechas, huecos y continuidad: desconocidos. El enunciado estima >3.5M filas largas.

## 1. Candidatas

| # | Pregunta | A quién le importa | Qué exige del dato (criba) | Qué aporta el K-Means | Qué la mata |
|---|---|---|---|---|---|
| Q1 | **¿Cuándo y dónde se concentran los episodios que exceden el ECA, y con qué meteorología llegan?** (hora, estación del año, estación de monitoreo; calma y humedad de noche = inversión; viento = polvo) | Población cercana a cada estación; municipios | PM2.5 o PM10 horarios con buena completitud; meteo en las mismas estaciones; al menos un año para ver estacionalidad | Regímenes = combinaciones típicas de PM y meteo; el «dónde y cuándo» sale de cruzar régimen con estación y hora | Que la respuesta sea trivial («de noche, en Lima»); que la meteo falte donde hay PM |
| Q2 | **¿De qué está hecho cada episodio: combustión o polvo?** (fracción fina PM2.5/PM10) | Quien decide la intervención: tráfico y quema frente a suelo, minería y viento | PM2.5 y PM10 **simultáneos** en la misma estación y hora; PM2.5 ≤ PM10 casi siempre | Regímenes por composición, no solo por nivel | Que pocas estaciones midan las dos PM a la vez |
| Q3 | **¿Con cuánta anticipación se puede avisar?** Cuando una estación entra a un régimen, cuántas horas dura y con qué probabilidad el promedio de las 24 h siguientes excede el ECA | Personas con enfermedad respiratoria; colegios; postas | Series **continuas** por estación (huecos pequeños); suficientes episodios para estimar proporciones | El régimen es el estado que se detecta en tiempo real; su persistencia histórica es la anticipación | Que los episodios sean tan raros (<0,5 % de horas) que no haya con qué estimar; huecos grandes justo en los episodios (ver Q9) |
| Q4 | **¿Cuánto del año respira fuera del ECA la población cercana a cada estación, y en qué regímenes?** (horas al año por estación, «carga de exposición») | Salud pública; comparación justa entre lugares | Completitud alta y parecida entre estaciones (si no, se compara cobertura, no aire) | Etiqueta de régimen por hora; el conteo es trivial pero el régimen dice el porqué | Cobertura desigual entre estaciones |
| Q5 | **¿Los episodios son regionales o locales?** Si varias estaciones cercanas entran al régimen contaminado a la vez es un fenómeno de cuenca (quema, polvo, meteo); si entra una sola, hay una fuente local, que es lo fiscalizable | **OEFA**: distinguir lo que se puede fiscalizar de lo que no; vecinos de la fuente | Varias estaciones **cercanas entre sí** (Callao, Lima, un corredor minero) con datos simultáneos | Régimen por estación y hora; la co-ocurrencia entre estaciones vecinas es el análisis | Que las 34 estaciones estén aisladas unas de otras |
| Q6 | **¿Los gases tienen la firma horaria de una fuente industrial?** (SO2 en fundiciones: picos por turno, por día de semana) | OEFA; vecinos de la fundición o el puerto | SO2, NO2 o CO con completitud en las estaciones industriales | Regímenes con gases; su distribución por hora y día de semana | Que los gases falten o estén en pocas estaciones |
| Q7 | **¿Qué parte del país y de la población tiene una estación cerca?** Una zona sin estación no es una zona con aire limpio | Todos los que viven lejos de una estación; es el ángulo ético del DD | Coordenadas de las 34 estaciones; población por distrito (INEI, externo) | Ninguno: es cobertura. Acompaña a cualquiera de las otras | Nada; siempre entra, como advertencia |
| Q8 | **¿Las estaciones de OEFA ven lo mismo que las de SENAMHI en Lima?** | Quien usa cualquiera de las dos redes para decidir | Estaciones de ambas redes a pocos km, mismo periodo | Comparar regímenes entre redes | Pocas o ninguna estación cercana entre redes. Va al TB2 |
| Q9 | **¿Los huecos del dato coinciden con los episodios?** Si las estaciones dejan de reportar cuando el aire está peor, el dato subestima el problema | Quien confía en el dato; OEFA; el DD (ética del dato) | Marcas de tiempo completas para contar huecos; PM alrededor de cada hueco | Ninguno directo; pero cambia la lectura de Q3 y Q4 | Que los huecos sean pocos y aleatorios (entonces es una línea del informe, no una pregunta) |

**Lectura de la tabla.** Q1 y Q3 son las que el enunciado espera para el caso 10 («clustering y
predicción de PM2.5/PM10 para alertas tempranas»). Q2 y Q5 son las que nadie más va a hacer y usan
lo que OEFA tiene y SENAMHI no: estaciones alrededor de fuentes fiscalizadas. Q4, Q7 y Q9 son
baratas y dan la mitad del informe de ética. Q6 depende de que haya gases. Q8 es del TB2.

**Propuesta [decisión mía]**: una pregunta **base** y un **diferencial**. Base = Q3 apoyada en Q1 (es
lo que la rúbrica va a buscar y se responde con K-Means más la persistencia de los regímenes).
Diferencial = Q5 o Q2, la que el dato permita (co-ocurrencia entre estaciones vecinas, o composición
fina/gruesa). Q7 y Q9 entran siempre como advertencias. La pregunta final se escribe en una frase que
nombre las dos cosas.

## 2. La criba: lo que el EDA del lunes tiene que medir

Cada medición cierra o abre candidatas. Salen a `reports/eda_criba.md`.

| Medición | Decide |
|---|---|
| Rango de fechas por estación y **años completos** | Q1 (estacionalidad), Q3, Q4 |
| Completitud estación × parámetro (tabla de 34 filas) | Q2 (ambas PM), Q6 (gases), Q4 (comparabilidad), y las features del K-Means |
| **Huecos**: longitud y distribución de los tramos sin datos por estación; PM en las 6 h anteriores a cada hueco frente al resto | Q9, y la viabilidad de Q3 |
| Frecuencia de horas con PM2.5 > 50 o PM10 > 100 por estación (y del promedio móvil de 24 h) | Q3 y Q4: si es <0,5 % en todas, no hay episodios que clusterizar |
| PM2.5 ≤ PM10: proporción de violaciones; distribución de la fracción fina por estación | Q2 |
| Distancias entre estaciones (UTM → km); grupos a menos de 10–15 km | Q5, Q8 |
| Ciclo diario y semanal de PM y de los gases por estación (perfil medio por hora y por día de semana) | Q1, Q6: si el ciclo horario lo explica casi todo, Q1 es trivial |
| Meteorología: qué estaciones tienen viento y humedad junto con PM | Q1 y las features M1 del K-Means |
| Coordenadas de las 34 estaciones y su departamento | Q7 |

Con esa tabla, el martes se escribe en la spec de la PC3 (§12) la pregunta elegida, con la medición
que la sostiene al lado. Lo que no pasó la criba se lista en el informe como «preguntas que este dato
no puede responder», que también es un hallazgo.
