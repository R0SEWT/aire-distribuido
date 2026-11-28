<!-- Generado por tablas.py desde protocolo/codec_test.go. No editar a mano. -->

::: {#lst-cable}
```json
{"tipo":"HOLA","version":1,"nodo":"colab-1","workers":2}
{"tipo":"CARGAR","rango":{"desde":0,"hasta":1000},"huella":"sha256:ab12"}
{"tipo":"LISTO","huella":"sha256:ab12","n":1000}
{"tipo":"ITERAR","iter":3,"centroides":[0.1,-1.5,2,0.25]}
{"tipo":"PARCIAL","iter":3,"sumas":[4.5,-9,30,2.5],"conteos":[600,400],"inercia":812.0625}
{"tipo":"FIN"}
```

Una sesión en el cable, tal cual: un mensaje JSON por línea (la salida que comprueba ExampleCodificador) (`protocolo/codec_test.go`, líneas 286–291)
:::
