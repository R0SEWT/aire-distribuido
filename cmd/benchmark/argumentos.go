package main

import (
	"os"
	"strings"
)

// argumentos devuelve los argumentos que hay que pasarle a flag, sin el nombre
// del programa. En Android (Termux) el binario se lanza a través del enlazador
// del sistema y recibe su propia ruta dos veces: os.Args[0] tal como se escribió
// ("./benchmark") y os.Args[1] como ruta absoluta. flag.Parse se detiene en el
// primer argumento que no es un flag y descartaba todos los demás en silencio.
// Si el primer argumento es el mismo archivo que el programa, se salta.
func argumentos(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	resto := args[1:]
	if len(resto) > 0 && mismoPrograma(args[0], resto[0]) {
		resto = resto[1:]
	}
	return resto
}

func mismoPrograma(programa, arg string) bool {
	if arg == programa {
		return true
	}
	if strings.HasPrefix(arg, "-") {
		return false
	}
	a, err1 := os.Stat(programa)
	b, err2 := os.Stat(arg)
	return err1 == nil && err2 == nil && os.SameFile(a, b)
}
