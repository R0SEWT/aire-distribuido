package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// Dos nodos en la misma máquina sin -nombre no pueden presentarse igual: el
// nombre por omisión lleva el PID.
func TestNombrePorOmisionLlevaElEquipoYElPID(t *testing.T) {
	got := nombrePorOmision()
	if !strings.HasSuffix(got, fmt.Sprintf("-%d", os.Getpid())) {
		t.Fatalf("nombre por omisión = %q; tiene que terminar en el PID %d", got, os.Getpid())
	}
	if equipo, err := os.Hostname(); err == nil && equipo != "" && !strings.HasPrefix(got, equipo+"-") {
		t.Fatalf("nombre por omisión = %q; tiene que empezar por el equipo %q", got, equipo)
	}
	if strings.TrimSuffix(got, fmt.Sprintf("-%d", os.Getpid())) == "" {
		t.Fatalf("nombre por omisión = %q; le falta el equipo", got)
	}
}
