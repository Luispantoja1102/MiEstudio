package api

import (
	"fmt"
	"testing"
	"time"
)

func claimBody(station string, branch int64) map[string]any {
	return map[string]any{"station_id": station, "name": "Caja", "branch_id": branch}
}

func TestPrintQueueFromPhoneToStation(t *testing.T) {
	p := newPOS(t)
	a := p.a
	printClaimWait, printClaimInterval = 30*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { printClaimWait, printClaimInterval = 12*time.Second, time.Second })

	p.openCash(t, p.admin, 0)
	sale := p.mustSale(t, p.cashier, map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}})
	if sale.BusinessName == "" || sale.BranchName == "" {
		t.Fatalf("el ticket necesita el encabezado del negocio: %+v", sale)
	}

	// sin estación conectada no se manda nada
	r := a.do(t, "POST", "/api/print/jobs", p.cashier, map[string]any{"sale_id": sale.ID})
	if r.Status != 409 || r.Body["code"] != "STATION_OFFLINE" {
		t.Fatalf("sin estación: %d %v", r.Status, r.Body)
	}
	r = a.do(t, "GET", fmt.Sprintf("/api/print/station?branch_id=%d", p.branch), p.cashier, nil)
	if r.Body["online"] != false {
		t.Fatalf("la estación no debería estar en línea: %v", r.Body)
	}

	// la estación se presenta (no hay trabajos: responde null) y queda en línea
	r = a.do(t, "POST", "/api/print/claim", p.admin, claimBody("st-1", p.branch))
	if r.Status != 200 || r.Body["job"] != nil {
		t.Fatalf("claim vacío: %d %v", r.Status, r.Body)
	}
	r = a.do(t, "GET", fmt.Sprintf("/api/print/station?branch_id=%d", p.branch), p.cashier, nil)
	if r.Body["online"] != true || r.Body["name"] != "Caja" {
		t.Fatalf("la estación debería estar en línea: %v", r.Body)
	}

	r = a.do(t, "POST", "/api/print/jobs", p.cashier, map[string]any{"sale_id": sale.ID, "open_drawer": true})
	if r.Status != 201 {
		t.Fatalf("mandar ticket: %d %v", r.Status, r.Body)
	}
	jobID := id(r)

	// la estación recibe la venta completa y el aviso del cajón
	r = a.do(t, "POST", "/api/print/claim", p.admin, claimBody("st-1", p.branch))
	job, _ := r.Body["job"].(map[string]any)
	if job == nil || job["open_drawer"] != true || job["sale"].(map[string]any)["folio"] != float64(sale.Folio) {
		t.Fatalf("claim: %d %v", r.Status, r.Body)
	}
	// otra estación no puede tomar un trabajo ya tomado, ni reportarlo
	r = a.do(t, "POST", "/api/print/claim", p.admin, claimBody("st-2", p.branch))
	if r.Body["job"] != nil {
		t.Fatalf("un trabajo lo toma una sola estación: %v", r.Body)
	}
	r = a.do(t, "POST", path("/api/print/jobs/", jobID, "/result"), p.admin, map[string]any{"station_id": "st-2", "ok": true})
	if r.Status != 409 {
		t.Fatalf("resultado de otra estación: %d", r.Status)
	}
	r = a.do(t, "POST", path("/api/print/jobs/", jobID, "/result"), p.admin, map[string]any{"station_id": "st-1", "ok": false, "error": "sin papel"})
	if r.Status != 200 {
		t.Fatalf("resultado: %d %v", r.Status, r.Body)
	}
	r = a.do(t, "GET", path("/api/print/jobs/", jobID), p.cashier, nil)
	if r.Body["status"] != "failed" || r.Body["error"] != "sin papel" {
		t.Fatalf("estado del trabajo: %v", r.Body)
	}
}

func TestPrintQueueIsolatedBetweenBusinesses(t *testing.T) {
	p := newPOS(t)
	a := p.a
	printClaimWait, printClaimInterval = 30*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { printClaimWait, printClaimInterval = 12*time.Second, time.Second })

	p.openCash(t, p.admin, 0)
	sale := p.mustSale(t, p.cashier, map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}})
	a.do(t, "POST", "/api/print/claim", p.admin, claimBody("st-1", p.branch))
	p.must(t, a.do(t, "POST", "/api/print/jobs", p.cashier, map[string]any{"sale_id": sale.ID}))

	other := a.mustRegister(t, "Otro", "otro@x.com")
	// no puede imprimir ni ver ventas ajenas, ni registrar una estación en una sucursal ajena
	if r := a.do(t, "POST", "/api/print/jobs", other, map[string]any{"sale_id": sale.ID}); r.Status != 404 {
		t.Fatalf("venta ajena: %d", r.Status)
	}
	if r := a.do(t, "POST", "/api/print/claim", other, claimBody("st-x", p.branch)); r.Status != 404 {
		t.Fatalf("sucursal ajena: %d %v", r.Status, r.Body)
	}
}
