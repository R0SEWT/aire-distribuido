<!-- Generado por tablas.py desde almacen/redis/redis.go. No editar a mano. -->

::: {#lst-guion-regimen}
```lua
local guardado = redis.call('GET', KEYS[1])
if guardado then
  local ok, doc = pcall(cjson.decode, guardado)
  if ok and type(doc) == 'table' and type(doc.fecha_hora) == 'string' and doc.fecha_hora > ARGV[2] then
    return 0
  end
end
redis.call('SET', KEYS[1], ARGV[1])
return 1
```

El script de Lua con que Redis compara y escribe el régimen de una estación en un solo paso (`almacen/redis/redis.go`, líneas 58–66)
:::
