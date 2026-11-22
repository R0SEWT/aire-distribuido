package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Roles del JWT. usuario predice y consulta; admin, además, ve las métricas y
// (más adelante) el clúster y los entrenamientos.
const (
	RolUsuario = "usuario"
	RolAdmin   = "admin"
)

// largoMinimoSecreto: HS256 firma con una clave de 256 bits; una más corta
// baja la seguridad de la firma a la de la clave (RFC 7518, §3.2).
const largoMinimoSecreto = 32

var (
	b64             = base64.RawURLEncoding
	cabeceraHS256   = b64.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	errTokenMalo    = errors.New("token inválido")
	errTokenVencido = errors.New("token expirado")
)

// Firmador emite y verifica JWT HS256 (RFC 7519) con crypto/hmac y
// crypto/sha256, sin dependencias. El secreto llega por variable de entorno
// (cmd/api) y nunca sale del servidor.
type Firmador struct {
	secreto  []byte
	duracion time.Duration
	ahora    func() time.Time
}

// Reclamos es la carga del token: quién es, con qué rol y hasta cuándo vale
// (segundos Unix, como pide el estándar).
type Reclamos struct {
	Usuario string `json:"sub"`
	Rol     string `json:"rol"`
	Emitido int64  `json:"iat"`
	Expira  int64  `json:"exp"`
}

// NuevoFirmador crea un firmador con tokens que valen duracion.
func NuevoFirmador(secreto []byte, duracion time.Duration) (*Firmador, error) {
	if len(secreto) < largoMinimoSecreto {
		return nil, fmt.Errorf("el secreto del JWT tiene %d bytes y hacen falta al menos %d (por ejemplo, openssl rand -hex 32)",
			len(secreto), largoMinimoSecreto)
	}
	if duracion <= 0 {
		return nil, fmt.Errorf("la duración del JWT tiene que ser positiva, es %v", duracion)
	}
	return &Firmador{secreto: append([]byte(nil), secreto...), duracion: duracion, ahora: time.Now}, nil
}

// Emitir firma un token para el usuario y el rol, y dice cuándo vence.
func (f *Firmador) Emitir(usuario, rol string) (string, time.Time, error) {
	ahora := f.ahora()
	r := Reclamos{Usuario: usuario, Rol: rol, Emitido: ahora.Unix(), Expira: ahora.Add(f.duracion).Unix()}
	carga, err := json.Marshal(r)
	if err != nil {
		return "", time.Time{}, err
	}
	sinFirma := cabeceraHS256 + "." + b64.EncodeToString(carga)
	return sinFirma + "." + b64.EncodeToString(f.firmar(sinFirma)), time.Unix(r.Expira, 0), nil
}

// Verificar comprueba el algoritmo, la firma (en tiempo constante, con
// hmac.Equal), el vencimiento y el rol, en ese orden: la carga no se lee antes
// de saber que la firmó este servidor.
func (f *Firmador) Verificar(token string) (Reclamos, error) {
	partes := strings.Split(token, ".")
	if len(partes) != 3 {
		return Reclamos{}, errTokenMalo
	}
	var cab struct {
		Alg string `json:"alg"`
	}
	if err := decodificarParte(partes[0], &cab); err != nil || cab.Alg != "HS256" {
		// Solo HS256: un token con alg = none o con otro algoritmo no se acepta.
		return Reclamos{}, errTokenMalo
	}
	firma, err := b64.DecodeString(partes[2])
	if err != nil || !hmac.Equal(firma, f.firmar(partes[0]+"."+partes[1])) {
		return Reclamos{}, errTokenMalo
	}
	var r Reclamos
	if err := decodificarParte(partes[1], &r); err != nil || r.Usuario == "" || r.Expira == 0 {
		return Reclamos{}, errTokenMalo
	}
	if !f.ahora().Before(time.Unix(r.Expira, 0)) {
		return Reclamos{}, errTokenVencido
	}
	if r.Rol != RolUsuario && r.Rol != RolAdmin {
		return Reclamos{}, errTokenMalo
	}
	return r, nil
}

func (f *Firmador) firmar(s string) []byte {
	mac := hmac.New(sha256.New, f.secreto)
	mac.Write([]byte(s))
	return mac.Sum(nil)
}

func decodificarParte(parte string, v any) error {
	b, err := b64.DecodeString(parte)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
