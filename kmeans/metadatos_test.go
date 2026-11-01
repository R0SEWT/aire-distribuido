package kmeans

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSha256ArchivoCoincideConElValorConocido(t *testing.T) {
	// sha256 de "abc", el vector de prueba clásico.
	ruta := filepath.Join(t.TempDir(), "abc.txt")
	if err := os.WriteFile(ruta, []byte("abc"), 0o600); err != nil {
		t.Fatalf("escribir el archivo: %v", err)
	}
	const esperado = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"

	h, err := Sha256Archivo(ruta)
	if err != nil {
		t.Fatalf("Sha256Archivo: %v", err)
	}
	if h != esperado {
		t.Errorf("hash = %s, se esperaba %s", h, esperado)
	}
}

func TestSha256ArchivoFallaSiNoExiste(t *testing.T) {
	if _, err := Sha256Archivo(filepath.Join(t.TempDir(), "no-existe")); err == nil {
		t.Fatal("se esperaba error con un archivo inexistente")
	}
}

func TestRecolectarMetadatosTraeLoQueElBenchmarkNecesita(t *testing.T) {
	m := RecolectarMetadatos()

	if m.VersionGo != runtime.Version() {
		t.Errorf("VersionGo = %q, se esperaba %q", m.VersionGo, runtime.Version())
	}
	if m.GOMAXPROCS != runtime.GOMAXPROCS(0) {
		t.Errorf("GOMAXPROCS = %d, se esperaba %d", m.GOMAXPROCS, runtime.GOMAXPROCS(0))
	}
	if m.CPUsLogicas != runtime.NumCPU() {
		t.Errorf("CPUsLogicas = %d, se esperaba %d", m.CPUsLogicas, runtime.NumCPU())
	}
	if m.Maquina == "" {
		t.Error("Maquina vacía: cada resultado tiene que decir dónde se midió")
	}
	if m.Momento.IsZero() {
		t.Error("Momento sin fecha")
	}
}

func TestMetadatosTraenElCommitAunqueSeCompileDesdeUnWorktree(t *testing.T) {
	// Go no incrusta vcs.revision cuando compila desde un worktree de git, y
	// sin commit el resultado del benchmark no se puede reproducir.
	m := RecolectarMetadatos()
	if m.Commit == "" {
		t.Skip("no hay git disponible o no estamos en un repo")
	}
	if len(m.Commit) != 40 {
		t.Errorf("Commit = %q, se esperaba un sha1 de 40 caracteres", m.Commit)
	}
}
