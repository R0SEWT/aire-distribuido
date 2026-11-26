package api

import (
	"fmt"
	"strings"
)

// Usuario es una credencial de demostración: la API no tiene registro de
// usuarios, las credenciales y el rol se configuran en el servidor.
type Usuario struct {
	Clave string
	Rol   string
}

// LeerUsuarios lee la lista de usuarios «usuario:clave:rol» separados por
// comas, como llega en la variable de entorno API_USUARIOS. Ni el usuario ni
// la clave pueden llevar «:» o «,».
func LeerUsuarios(s string) (map[string]Usuario, error) {
	us := map[string]Usuario{}
	for _, entrada := range strings.Split(s, ",") {
		partes := strings.Split(strings.TrimSpace(entrada), ":")
		if len(partes) != 3 || partes[0] == "" || partes[1] == "" {
			return nil, fmt.Errorf("usuarios: %q no tiene la forma usuario:clave:rol", strings.TrimSpace(entrada))
		}
		nombre, clave, rol := partes[0], partes[1], partes[2]
		if rol != RolUsuario && rol != RolAdmin {
			return nil, fmt.Errorf("usuarios: el rol de %s es %q, tiene que ser %s o %s", nombre, rol, RolUsuario, RolAdmin)
		}
		if _, ok := us[nombre]; ok {
			return nil, fmt.Errorf("usuarios: %s está repetido", nombre)
		}
		us[nombre] = Usuario{Clave: clave, Rol: rol}
	}
	return us, nil
}

// ValidarInvitado comprueba la cuenta del ingreso de invitado (Config.Invitado,
// la variable de entorno API_INVITADO): tiene que ser uno de los usuarios y
// tener el rol usuario. Quien entra como invitado no presenta ninguna clave,
// así que nunca puede ser un admin. Con "" no hay invitado y no hay nada que
// comprobar.
func ValidarInvitado(usuarios map[string]Usuario, invitado string) error {
	if invitado == "" {
		return nil
	}
	u, existe := usuarios[invitado]
	switch {
	case !existe:
		return fmt.Errorf("el invitado %q no está entre los usuarios: tiene que ser uno de API_USUARIOS, con el rol %s", invitado, RolUsuario)
	case u.Rol != RolUsuario:
		return fmt.Errorf("el invitado %q tiene el rol %s: entra sin clave, así que tiene que ser un usuario con el rol %s", invitado, u.Rol, RolUsuario)
	}
	return nil
}
