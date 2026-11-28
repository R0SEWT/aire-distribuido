// Esta carpeta no es Go. El go.mod la separa del módulo del repo para que `go test ./...` y
// `go vet ./...` no entren a node_modules (flatted trae un paquete de Go en node_modules/flatted/golang).
module aire-distribuido/docs/video/remotion

go 1.26
