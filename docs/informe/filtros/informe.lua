--[[
Filtro del informe (Quarto → docx). Hace siete cosas:

1. Carátula según el código de alumno. `codigo` es el parámetro del render
   (`quarto render informe.qmd -M codigo:<código>`); se busca en `equipo` (generado/local/equipo.yml) y:
     [ ]{.alumno-codigo}  → el código
     [ ]{.alumno-nombre}  → el nombre de ese integrante
     ::: {.caratula-integrantes} ::: → una línea por integrante, «Nombre (código)»
   Un código que no está en `equipo` detiene el render: así no sale un informe con una carátula
   equivocada.
2. Índice. `::: indice` ::: se reemplaza por el campo TOC de Word (el mismo que genera Pandoc con
   `toc: true`), pero en ese lugar: Pandoc pondría su índice antes de la carátula, y el enunciado
   ordena carátula, resumen, índice. Word lo llena al abrir el archivo (pide actualizar campos).
3. Figuras que todavía no existen: la imagen se reemplaza por recursos/figura-pendiente.png y se
   avisa por stderr. El render no falla por una figura que otro integrante aún no produce.
4. Avisos condicionales: un div con `si-falta="ruta"` se elimina si la ruta existe. Se usa para el
   aviso PENDIENTE de cada figura: desaparece solo cuando la figura llega.
     :::: {si-falta="generado/figuras/x.png"}
     ::: {.callout-warning title="PENDIENTE"}
     ...
     :::
     ::::
   (El atributo va en un div exterior: Quarto convierte los callouts antes de que corra este filtro.)
5. Tablas, listados y algoritmos con id alineados a la izquierda. En docx, Quarto centra el
   contenido de un flotante, y con él el texto de cada celda de una tabla (pisando la alineación de
   las columnas) y cada línea de un bloque de código (que pierde la sangría). Con
   `tbl-align="left"`, `lst-align="left"` o `alg-align="left"` se respeta la del contenido. La
   opción global no se toma de los metadatos, así que se pone flotante por flotante, salvo que ya
   traiga la suya. Las figuras siguen centradas.
6. Separadores entre tablas. En docx, Quarto dibuja figuras, tablas y avisos como tablas de Word, y
   dos tablas sin un párrafo entre ellas se funden en una sola (Word y LibreOffice las unen). Entre
   dos bloques así se inserta un párrafo vacío mínimo.
7. Figuras dentro del ancho de la celda. En docx, Quarto pone cada figura en una tabla de una celda
   con 0,19 cm de margen a cada lado (estilo «Table» de la plantilla), así que el ancho útil es
   16 − 0,38 cm. Una imagen de 16 cm se sale y LibreOffice la recorta por la derecha. Todo ancho en
   cm se limita a ANCHO_MAX_CM.
]]

local base = nil
local function dir_base()
  if base == nil then
    local entrada = quarto and quarto.doc and quarto.doc.input_file or nil
    base = entrada and pandoc.path.directory(entrada) or "."
  end
  return base
end

local function existe(ruta)
  if ruta == nil or ruta == "" or ruta:match("^%a+://") or ruta:match("^data:") then
    return true
  end
  if not pandoc.path.is_absolute(ruta) then
    ruta = pandoc.path.join({ dir_base(), ruta })
  end
  local f = io.open(ruta, "rb")
  if f then
    f:close()
    return true
  end
  return false
end

local CODIGO, NOMBRE = nil, nil
local INTEGRANTES = {}
local NIVELES = "1-3"
local TITULO_INDICE = "Índice"

local function leer_meta(meta)
  if meta.equipo then
    for _, m in ipairs(meta.equipo) do
      table.insert(INTEGRANTES, {
        nombre = pandoc.utils.stringify(m.nombre or ""),
        codigo = pandoc.utils.stringify(m.codigo or ""),
        rol = m.rol and pandoc.utils.stringify(m.rol) or nil,
      })
    end
  end
  if meta.codigo then
    CODIGO = pandoc.utils.stringify(meta.codigo)
    for _, m in ipairs(INTEGRANTES) do
      if m.codigo == CODIGO then
        NOMBRE = m.nombre
      end
    end
    if NOMBRE == nil and #INTEGRANTES > 0 then
      -- Quarto registra un error() de un filtro y sigue: os.exit sí detiene el render.
      io.stderr:write("error: codigo " .. CODIGO .. " no está en `equipo` (generado/local/equipo.yml); "
        .. "no se genera un informe con una carátula equivocada\n")
      os.exit(1)
    end
  end
  if meta["indice-niveles"] then
    NIVELES = pandoc.utils.stringify(meta["indice-niveles"])
  end
  if meta["indice-titulo"] then
    TITULO_INDICE = pandoc.utils.stringify(meta["indice-titulo"])
  end
end

local function escapar_xml(s)
  return (s:gsub("&", "&amp;"):gsub("<", "&lt;"):gsub(">", "&gt;"):gsub('"', "&quot;"))
end

local function indice()
  local xml = '<w:sdt><w:sdtPr><w:docPartObj><w:docPartGallery w:val="Table of Contents"/>'
    .. '<w:docPartUnique/></w:docPartObj></w:sdtPr><w:sdtContent>'
    .. '<w:p><w:pPr><w:pStyle w:val="TOCHeading"/></w:pPr><w:r><w:t xml:space="preserve">'
    .. escapar_xml(TITULO_INDICE) .. '</w:t></w:r></w:p>'
    .. '<w:p><w:r><w:fldChar w:fldCharType="begin" w:dirty="true"/>'
    .. '<w:instrText xml:space="preserve">TOC \\o &quot;' .. NIVELES .. '&quot; \\h \\z \\u</w:instrText>'
    .. '<w:fldChar w:fldCharType="separate"/></w:r>'
    .. '<w:r><w:rPr><w:i/></w:rPr><w:t xml:space="preserve">'
    .. 'El índice se genera al abrir el documento en Word (Actualizar campos). '
    .. 'Si no aparece: clic derecho aquí y «Actualizar campo».</w:t></w:r>'
    .. '<w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>'
    .. '</w:sdtContent></w:sdt>'
  return pandoc.RawBlock("openxml", xml)
end

local function integrantes()
  local bloques = {}
  for _, m in ipairs(INTEGRANTES) do
    local texto = m.nombre .. " (" .. m.codigo .. ")"
    if m.rol then
      texto = texto .. ", " .. m.rol
    end
    table.insert(bloques, pandoc.Para(pandoc.Inlines(texto)))
  end
  return bloques
end

local function Span(el)
  if el.classes:includes("alumno-codigo") then
    return pandoc.Str(CODIGO or "UXXXXXXXXX")
  end
  if el.classes:includes("alumno-nombre") then
    return pandoc.Inlines(NOMBRE or "(sin código de alumno)")
  end
end

local function Div(el)
  local ruta = el.attributes["si-falta"]
  if ruta then
    if existe(ruta) then
      return {}
    end
    el.attributes["si-falta"] = nil
    return el
  end
  if el.classes:includes("indice") then
    return indice()
  end
  if el.classes:includes("caratula-integrantes") then
    el.content = integrantes()
    return el
  end
end

-- Prefijos de los flotantes que van a la izquierda; Quarto lee «<prefijo>-align» del id.
local A_LA_IZQUIERDA = { tbl = true, lst = true, alg = true }

local function FloatRefTarget(f)
  local prefijo = (f.identifier or ""):match("^(%a+)-")
  if prefijo and A_LA_IZQUIERDA[prefijo] and f.attributes[prefijo .. "-align"] == nil then
    f.attributes[prefijo .. "-align"] = "left"
    return f
  end
end

-- Ancho útil de la celda de un flotante: 16 cm de texto menos 2 × 0,19 cm de margen, con holgura.
local ANCHO_MAX_CM = 15.5

local function Image(img)
  if not existe(img.src) then
    io.stderr:write("aviso: falta la figura " .. img.src .. "; se usa recursos/figura-pendiente.png\n")
    img.src = "recursos/figura-pendiente.png"
  end
  local cm = tonumber((img.attributes.width or ""):match("^([%d.]+)cm$"))
  if cm and cm > ANCHO_MAX_CM then
    img.attributes.width = ANCHO_MAX_CM .. "cm"
  end
  return img
end

-- Bloques que en docx terminan siendo una tabla de Word: tablas, y los nodos de Quarto para
-- flotantes (figuras y tablas con id) y avisos. Un div común cuenta por su primer o último bloque.
local TABULARES = { FloatRefTarget = true, Callout = true, PanelLayout = true }

local function es_tabular(b, extremo)
  if b == nil then
    return false
  end
  if b.t == "Table" or b.t == "Figure" then
    return true
  end
  if b.t == "RawBlock" and b.format == "openxml" then
    return b.text:match("^%s*<w:tbl") ~= nil
  end
  if b.t == "Div" then
    local tipo = b.attributes["__quarto_custom_type"]
    if tipo then
      return TABULARES[tipo] == true
    end
    if #b.content > 0 then
      return es_tabular(extremo == "inicio" and b.content[1] or b.content[#b.content], extremo)
    end
  end
  return false
end

local function separador()
  return pandoc.RawBlock("openxml",
    '<w:p><w:pPr><w:spacing w:before="0" w:after="0" w:line="120" w:lineRule="exact"/></w:pPr></w:p>')
end

-- Un bloque crudo que no es openxml (un comentario HTML, p. ej.) no existe en el docx: no separa.
local function invisible(b)
  return b.t == "RawBlock" and b.format ~= "openxml"
end

local function Blocks(bloques)
  local salida = pandoc.Blocks({})
  local anterior = nil
  for _, b in ipairs(bloques) do
    if not invisible(b) then
      if es_tabular(anterior, "fin") and es_tabular(b, "inicio") then
        salida:insert(separador())
      end
      anterior = b
    end
    salida:insert(b)
  end
  return salida
end

return {
  { Meta = function(meta) leer_meta(meta) end },
  { Span = Span, Div = Div, Image = Image, FloatRefTarget = FloatRefTarget },
  { Blocks = Blocks },
}
