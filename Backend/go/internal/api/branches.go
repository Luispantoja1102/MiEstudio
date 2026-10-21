package api

import (
	"net/http"
	"strings"

	"miestudio/backend-go/internal/auth"
)

type branch struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Phone   string `json:"phone"`
	Active  bool   `json:"active"`
}

func (a *API) listBranches(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rows, err := a.DB.Query(r.Context(),
		`SELECT id, name, address, phone, active FROM branches WHERE business_id = $1 ORDER BY id`, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []branch{}
	for rows.Next() {
		var b branch
		if err := rows.Scan(&b.ID, &b.Name, &b.Address, &b.Phone, &b.Active); err != nil {
			serverErr(w)
			return
		}
		out = append(out, b)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createBranch(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct{ Name, Address, Phone string }
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !validName(req.Name) || len(req.Address) > 300 || len(req.Phone) > 30 {
		writeErr(w, http.StatusBadRequest, "datos de sucursal inválidos")
		return
	}
	if err := a.checkBranchRoom(r.Context(), p.BusinessID); err != nil {
		writeFail(w, err)
		return
	}
	var id int64
	if err := a.DB.QueryRow(r.Context(),
		`INSERT INTO branches (business_id, name, address, phone) VALUES ($1,$2,$3,$4) RETURNING id`,
		p.BusinessID, req.Name, req.Address, req.Phone).Scan(&id); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *API) updateBranch(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Name    *string `json:"name"`
		Address *string `json:"address"`
		Phone   *string `json:"phone"`
		Active  *bool   `json:"active"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if req.Name != nil {
		t := strings.TrimSpace(*req.Name)
		req.Name = &t
		if !validName(t) {
			writeErr(w, http.StatusBadRequest, "el nombre es obligatorio")
			return
		}
	}
	if (req.Address != nil && len(*req.Address) > 300) || (req.Phone != nil && len(*req.Phone) > 30) {
		writeErr(w, http.StatusBadRequest, "datos de sucursal inválidos")
		return
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)

	if req.Active != nil && *req.Active {
		var wasActive bool
		if err := tx.QueryRow(ctx, `SELECT active FROM branches WHERE id = $1 AND business_id = $2`, id, p.BusinessID).Scan(&wasActive); err == nil && !wasActive {
			if err := a.checkBranchRoom(ctx, p.BusinessID); err != nil {
				writeFail(w, err)
				return
			}
		}
	}
	if req.Active != nil && !*req.Active {
		// debe quedar al menos otra sucursal activa
		var others int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM branches WHERE business_id = $1 AND active AND id <> $2`, p.BusinessID, id).Scan(&others); err != nil {
			serverErr(w)
			return
		}
		if others == 0 {
			writeErr(w, http.StatusBadRequest, "debe quedar al menos una sucursal activa")
			return
		}
	}
	tag, err := tx.Exec(ctx,
		`UPDATE branches SET name = COALESCE($3, name), address = COALESCE($4, address),
		   phone = COALESCE($5, phone), active = COALESCE($6, active)
		 WHERE id = $1 AND business_id = $2`, id, p.BusinessID, req.Name, req.Address, req.Phone, req.Active)
	if err != nil {
		serverErr(w)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
