package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var secretoDePrueba = []byte("un-secreto-de-prueba-de-32-bytes!!")

func firmadorDePrueba(ahora time.Time) *Firmador {
	f, err := NuevoFirmador(secretoDePrueba, time.Hour)
	if err != nil {
		panic(err)
	}
	f.ahora = func() time.Time { return ahora }
	return f
}

func TestJWTEmitirYVerificar(t *testing.T) {
	t0 := time.Date(2026, 11, 19, 15, 0, 0, 0, time.UTC)
	f := firmadorDePrueba(t0)
	tok, expira, err := f.Emitir("demo", RolUsuario)
	if err != nil {
		t.Fatal(err)
	}
	if !expira.Equal(t0.Add(time.Hour)) {
		t.Fatalf("expira %v, se esperaba %v", expira, t0.Add(time.Hour))
	}
	partes := strings.Split(tok, ".")
	if len(partes) != 3 {
		t.Fatalf("el token no tiene tres partes: %q", tok)
	}
	var cab map[string]string
	decodificar(t, partes[0], &cab)
	if cab["alg"] != "HS256" || cab["typ"] != "JWT" {
		t.Fatalf("cabecera %v", cab)
	}
	r, err := f.Verificar(tok)
	if err != nil {
		t.Fatalf("Verificar: %v", err)
	}
	if r.Usuario != "demo" || r.Rol != RolUsuario || r.Expira != t0.Add(time.Hour).Unix() {
		t.Fatalf("reclamos %+v", r)
	}
}

// La firma es HMAC-SHA256 de «cabecera.carga» en base64url sin relleno (RFC
// 7519): un token armado a mano con la misma clave tiene que valer.
func TestJWTEsElEstandar(t *testing.T) {
	t0 := time.Unix(1_763_000_000, 0)
	f := firmadorDePrueba(t0)
	b64 := base64.RawURLEncoding
	cab := b64.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	carga := b64.EncodeToString([]byte(`{"sub":"ana","rol":"admin","iat":1763000000,"exp":1763000600}`))
	mac := hmac.New(sha256.New, secretoDePrueba)
	mac.Write([]byte(cab + "." + carga))
	tok := cab + "." + carga + "." + b64.EncodeToString(mac.Sum(nil))
	r, err := f.Verificar(tok)
	if err != nil || r.Usuario != "ana" || r.Rol != RolAdmin {
		t.Fatalf("Verificar = %+v, %v", r, err)
	}
}

func TestJWTRechaza(t *testing.T) {
	t0 := time.Date(2026, 11, 19, 15, 0, 0, 0, time.UTC)
	f := firmadorDePrueba(t0)
	valido, _, _ := f.Emitir("demo", RolUsuario)
	partes := strings.Split(valido, ".")
	b64 := base64.RawURLEncoding
	firmar := func(cab, carga string) string {
		c, p := b64.EncodeToString([]byte(cab)), b64.EncodeToString([]byte(carga))
		mac := hmac.New(sha256.New, secretoDePrueba)
		mac.Write([]byte(c + "." + p))
		return c + "." + p + "." + b64.EncodeToString(mac.Sum(nil))
	}
	otroSecreto, _ := NuevoFirmador([]byte("otro-secreto-de-prueba-de-32-bytes"), time.Hour)
	deOtro, _, _ := otroSecreto.Emitir("demo", RolUsuario)
	caducado := firmadorDePrueba(t0.Add(-2 * time.Hour))
	viejo, _, _ := caducado.Emitir("demo", RolUsuario)

	casos := map[string]string{
		"vacío":                  "",
		"dos partes":             partes[0] + "." + partes[1],
		"firma alterada":         partes[0] + "." + partes[1] + "." + b64.EncodeToString([]byte("no es la firma")),
		"carga alterada":         partes[0] + "." + b64.EncodeToString([]byte(`{"sub":"demo","rol":"admin","exp":9999999999}`)) + "." + partes[2],
		"firmado con otra clave": deOtro,
		"caducado":               viejo,
		"alg none":               b64.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + partes[1] + ".",
		"alg HS512":              firmar(`{"alg":"HS512","typ":"JWT"}`, `{"sub":"demo","rol":"usuario","exp":9999999999}`),
		"sin exp":                firmar(`{"alg":"HS256","typ":"JWT"}`, `{"sub":"demo","rol":"usuario"}`),
		"rol desconocido":        firmar(`{"alg":"HS256","typ":"JWT"}`, `{"sub":"demo","rol":"root","exp":9999999999}`),
		"carga que no es JSON":   firmar(`{"alg":"HS256","typ":"JWT"}`, `no es json`),
	}
	for nombre, tok := range casos {
		t.Run(nombre, func(t *testing.T) {
			if r, err := f.Verificar(tok); err == nil {
				t.Fatalf("Verificar aceptó el token: %+v", r)
			}
		})
	}
}

func TestNuevoFirmadorExigeUnSecretoLargo(t *testing.T) {
	if _, err := NuevoFirmador([]byte("corto"), time.Hour); err == nil {
		t.Fatal("aceptó un secreto de 5 bytes")
	}
	if _, err := NuevoFirmador(secretoDePrueba, 0); err == nil {
		t.Fatal("aceptó una duración 0")
	}
}

func decodificar(t *testing.T, parte string, v any) {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(parte)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}
