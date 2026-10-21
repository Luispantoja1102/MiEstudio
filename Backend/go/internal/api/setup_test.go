package api

import "testing"

func pendingFor(t *testing.T, a *API, token string) any {
	t.Helper()
	b, _ := a.do(t, "GET", "/api/me", token, nil).Body["business"].(map[string]any)
	return b["setup_pending"]
}

func TestSetupWizardIsShownOnlyToTheOwnerOfANewBusiness(t *testing.T) {
	a := newTestAPI(t)
	owner := a.mustRegister(t, "Barbella Centro", "dueno@x.com")
	if pendingFor(t, a, owner) != true {
		t.Fatal("un negocio nuevo debe mostrar el asistente a su titular")
	}
	a.addMember(t, owner, "socio@x.com", "admin")
	a.addMember(t, owner, "staff@x.com", "staff")
	for _, email := range []string{"socio@x.com", "staff@x.com"} {
		tok := a.doLogin(t, email, "supersecreta1").Body["token"].(string)
		if pendingFor(t, a, tok) != false {
			t.Fatalf("%s no es titular y no debe ver el asistente", email)
		}
		if email == "staff@x.com" {
			if r := a.do(t, "POST", "/api/business/setup/complete", tok, nil); r.Status != 403 {
				t.Fatalf("el personal no puede cerrar el asistente: %d", r.Status)
			}
		}
	}
	if r := a.do(t, "POST", "/api/business/setup/complete", owner, nil); r.Status != 204 {
		t.Fatalf("completar: %d %v", r.Status, r.Body)
	}
	if pendingFor(t, a, owner) != false {
		t.Fatal("tras completarlo ya no se muestra")
	}
	// repetirlo no falla ni lo reabre
	if r := a.do(t, "POST", "/api/business/setup/complete", owner, nil); r.Status != 204 || pendingFor(t, a, owner) != false {
		t.Fatalf("repetir: %d", r.Status)
	}
}

func TestSetupWizardIsNotShownToBusinessesThatAlreadyExisted(t *testing.T) {
	a := newTestAPI(t)
	owner := a.mustRegister(t, "Antiguo", "viejo@x.com")
	// como dejó la migración a los negocios previos
	if _, err := a.DB.Exec(t.Context(), `UPDATE businesses SET setup_completed_at = now()`); err != nil {
		t.Fatal(err)
	}
	if pendingFor(t, a, owner) != false {
		t.Fatal("un negocio ya configurado no debe ver el asistente")
	}
}
