package api

import "testing"

func TestLeerUsuarios(t *testing.T) {
	us, err := LeerUsuarios(" demo:demo:usuario , ana:una clave larga:admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(us) != 2 || us["demo"] != (Usuario{Clave: "demo", Rol: RolUsuario}) ||
		us["ana"] != (Usuario{Clave: "una clave larga", Rol: RolAdmin}) {
		t.Fatalf("usuarios = %+v", us)
	}
	for _, malo := range []string{
		"", "demo:demo", "demo::usuario", ":demo:usuario", "demo:demo:root",
		"demo:demo:usuario,demo:otra:admin", "demo:demo:usuario,,ana:x:admin",
	} {
		if us, err := LeerUsuarios(malo); err == nil {
			t.Errorf("LeerUsuarios(%q) = %+v, se esperaba un error", malo, us)
		}
	}
}
