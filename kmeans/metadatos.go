package kmeans

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// Metadatos acompaña a todo resultado. Sin esto una tabla de tiempos no se
// puede reproducir ni auditar: no se sabe en qué máquina, con qué Go, con qué
// commit ni sobre qué datos salió.
type Metadatos struct {
	Momento     time.Time `json:"momento"`
	Maquina     string    `json:"maquina"`
	VersionGo   string    `json:"version_go"`
	GOMAXPROCS  int       `json:"gomaxprocs"`
	CPUsLogicas int       `json:"cpus_logicas"`
	Commit      string    `json:"commit,omitempty"`
	ArbolSucio  bool      `json:"arbol_sucio"`
}

// RecolectarMetadatos lee lo que el binario sabe de sí mismo y de la máquina.
// El commit sale de la información de build que Go incrusta al compilar desde
// un repo git; con `go run` puede venir vacío, y entonces el harness debe
// completarlo.
func RecolectarMetadatos() Metadatos {
	m := Metadatos{
		Momento:     time.Now(),
		VersionGo:   runtime.Version(),
		GOMAXPROCS:  runtime.GOMAXPROCS(0),
		CPUsLogicas: runtime.NumCPU(),
	}
	if h, err := os.Hostname(); err == nil {
		m.Maquina = h
	} else {
		m.Maquina = "desconocida"
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				m.Commit = s.Value
			case "vcs.modified":
				m.ArbolSucio = s.Value == "true"
			}
		}
	}
	if m.Commit == "" {
		// Go no incrusta la revisión cuando compila desde un worktree de git
		// (ahí .git es un archivo, no un directorio). Como sin commit el
		// resultado no se puede reproducir, se pregunta directamente.
		m.Commit, m.ArbolSucio = commitDeGit()
	}
	return m
}

func commitDeGit() (string, bool) {
	rev, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", false
	}
	sucio := false
	if estado, err := exec.Command("git", "status", "--porcelain").Output(); err == nil {
		sucio = len(strings.TrimSpace(string(estado))) > 0
	}
	return strings.TrimSpace(string(rev)), sucio
}

// Sha256Archivo identifica los datos de entrada de una corrida.
func Sha256Archivo(ruta string) (string, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
