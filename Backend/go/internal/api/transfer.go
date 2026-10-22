package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"miestudio/backend-go/internal/auth"
)

// Anticipo por transferencia. La cita se aparta como «pending_payment»; el cliente (o el personal, desde la agenda)
// sube el comprobante o lo manda por WhatsApp, y el titular confirma la cita al verlo. Nada se da por pagado solo.

const (
	maxProofBytes    = 5 << 20
	proofReviewHours = 48 // con el comprobante ya enviado, el horario sigue apartado mientras el titular lo revisa
)

// depositMethods dice cómo puede pagar el anticipo quien reserva: Mercado Pago y/o transferencia.
func (a *API) depositMethods(ctx context.Context, b *pubBiz) []string {
	if b.Settings.DepositPercent <= 0 || !a.planOf(ctx, a.DB, b.ID).Deposits {
		return nil
	}
	out := []string{}
	if a.mpConnected(ctx, b.ID) {
		out = append(out, "mp")
	}
	if b.Settings.TransferEnabled {
		out = append(out, "transfer")
	}
	return out
}

// transferDetails es lo que ve quien debe pagar: a dónde transferir y con qué concepto.
func transferDetails(s bookingSettings, apptID int64) map[string]any {
	return map[string]any{
		"bank": s.TransferBank, "holder": s.TransferHolder, "clabe": s.TransferClabe,
		"whatsapp": waDigits(s.TransferWhatsApp), "reference": fmt.Sprintf("CITA-%d", apptID),
	}
}

// readProof lee y valida el archivo del comprobante (JPG, PNG, WebP o PDF de hasta 5 MB).
func readProof(w http.ResponseWriter, r *http.Request) (data []byte, ctype, name string, ok bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxProofBytes+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "el archivo pesa más de 5 MB o no se pudo leer")
		return nil, "", "", false
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "adjunta el comprobante")
		return nil, "", "", false
	}
	defer f.Close()
	data, err = io.ReadAll(io.LimitReader(f, maxProofBytes+1))
	if err != nil || len(data) == 0 {
		writeErr(w, http.StatusBadRequest, "no se pudo leer el archivo")
		return nil, "", "", false
	}
	if len(data) > maxProofBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "el archivo pesa más de 5 MB")
		return nil, "", "", false
	}
	switch ctype = http.DetectContentType(data); ctype {
	case "image/jpeg", "image/png", "image/webp", "application/pdf":
	default:
		writeErr(w, http.StatusUnsupportedMediaType, "usa una foto JPG, PNG o WebP, o un PDF")
		return nil, "", "", false
	}
	name = hdr.Filename
	if utf8.RuneCountInString(name) > 120 {
		name = string([]rune(name)[:120])
	}
	return data, ctype, name, true
}

// saveProof guarda el comprobante de una cita que espera transferencia. Devuelve false si la cita no lo espera.
func (a *API) saveProof(ctx context.Context, bizID, apptID int64, data []byte, ctype, name string) (bool, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var depID int64
	err = tx.QueryRow(ctx,
		`SELECT d.id FROM deposits d JOIN appointments ap ON ap.id = d.appointment_id
		  WHERE ap.id = $1 AND ap.business_id = $2 AND d.method = 'transfer' AND d.status = 'pending' AND ap.status = 'pending_payment' FOR UPDATE OF d, ap`, apptID, bizID).Scan(&depID)
	if err != nil {
		return false, nil
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO deposit_proofs (deposit_id, content_type, filename, data, uploaded_at) VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (deposit_id) DO UPDATE SET content_type = EXCLUDED.content_type, filename = EXCLUDED.filename, data = EXCLUDED.data, uploaded_at = EXCLUDED.uploaded_at`,
		depID, ctype, name, data, a.now()); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE deposits SET proof_at = $2, reject_reason = '' WHERE id = $1`, depID, a.now()); err != nil {
		return false, err
	}
	// mientras el titular lo revisa, el horario no se libera solo
	if _, err := tx.Exec(ctx,
		`UPDATE appointments SET hold_expires_at = GREATEST(COALESCE(hold_expires_at, $2), $2) WHERE id = $1`, apptID, a.now().Add(proofReviewHours*time.Hour)); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// publicProof es POST /api/public-appointments/{token}/proof: el cliente sube su comprobante.
func (a *API) publicProof(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !a.allow(w, "pub-proof:"+clientIP(r), 10, time.Hour) {
		return
	}
	m, err := a.byToken(ctx, r.PathValue("token"))
	if err != nil {
		serverErr(w)
		return
	}
	if m == nil {
		notFound(w)
		return
	}
	data, ctype, name, ok := readProof(w, r)
	if !ok {
		return
	}
	saved, err := a.saveProof(ctx, m.BizID, m.ID, data, ctype, name)
	if err != nil {
		serverErr(w)
		return
	}
	if !saved {
		writeErr(w, http.StatusConflict, "esta cita ya no espera un comprobante")
		return
	}
	a.inboxAppt(ctx, m.ID, "proof", "Comprobante recibido: revísalo y confirma")
	w.WriteHeader(http.StatusNoContent)
}

// staffProof es POST /api/appointments/{id}/proof: el personal adjunta el comprobante que el cliente mandó por WhatsApp.
func (a *API) staffProof(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	data, ctype, name, ok := readProof(w, r)
	if !ok {
		return
	}
	saved, err := a.saveProof(r.Context(), p.BusinessID, id, data, ctype, name)
	if err != nil {
		serverErr(w)
		return
	}
	if !saved {
		writeErr(w, http.StatusConflict, "esta cita no espera un comprobante")
		return
	}
	a.inboxAppt(r.Context(), id, "proof", "Comprobante recibido: revísalo y confirma")
	w.WriteHeader(http.StatusNoContent)
}

// getProof es GET /api/appointments/{id}/proof: el archivo, para quien administra.
func (a *API) getProof(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var data []byte
	var ctype, name string
	err := a.DB.QueryRow(r.Context(),
		`SELECT pr.data, pr.content_type, pr.filename FROM deposit_proofs pr JOIN deposits d ON d.id = pr.deposit_id
		  WHERE d.appointment_id = $1 AND d.business_id = $2`, id, p.BusinessID).Scan(&data, &ctype, &name)
	if err != nil {
		notFound(w)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", "inline")
	w.Write(data)
}

// confirmTransfer es POST /api/appointments/{id}/transfer/confirm: el titular vio el pago y confirma la cita.
func (a *API) confirmTransfer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	var depID int64
	if err := tx.QueryRow(ctx,
		`SELECT d.id FROM deposits d JOIN appointments ap ON ap.id = d.appointment_id
		  WHERE ap.id = $1 AND ap.business_id = $2 AND d.method = 'transfer' AND d.status = 'pending' AND ap.status = 'pending_payment' FOR UPDATE OF d, ap`, id, p.BusinessID).Scan(&depID); err != nil {
		writeErr(w, http.StatusConflict, "esta cita no espera una transferencia")
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE deposits SET status = 'paid', paid_at = $2, confirmed_by = $3, reject_reason = '' WHERE id = $1`, depID, a.now(), p.UserID); err != nil {
		serverErr(w)
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE appointments SET status = 'scheduled', hold_expires_at = NULL WHERE id = $1`, id); err != nil {
		serverErr(w)
		return
	}
	if err := a.enqueueMail(ctx, tx, id, "booking", false); err != nil {
		serverErr(w)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	a.logActivity(ctx, p.UserID, p.BusinessID, "transfer_confirmed", map[string]any{"appointment_id": id})
	w.WriteHeader(http.StatusNoContent)
}

// rejectTransfer es POST /api/appointments/{id}/transfer/reject: el comprobante no sirve; el cliente puede mandar otro.
func (a *API) rejectTransfer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if n := utf8.RuneCountInString(req.Reason); n < 3 || n > 200 {
		writeErr(w, http.StatusBadRequest, "escribe el motivo (de 3 a 200 caracteres) para que el cliente sepa qué corregir")
		return
	}
	st, err := loadBookingSettings(ctx, a.DB, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	var depID int64
	if err := tx.QueryRow(ctx,
		`SELECT d.id FROM deposits d JOIN appointments ap ON ap.id = d.appointment_id
		  WHERE ap.id = $1 AND ap.business_id = $2 AND d.method = 'transfer' AND d.status = 'pending' AND ap.status = 'pending_payment' FOR UPDATE OF d, ap`, id, p.BusinessID).Scan(&depID); err != nil {
		writeErr(w, http.StatusConflict, "esta cita no espera una transferencia")
		return
	}
	if _, err := tx.Exec(ctx, `DELETE FROM deposit_proofs WHERE deposit_id = $1`, depID); err != nil {
		serverErr(w)
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE deposits SET proof_at = NULL, reject_reason = $2 WHERE id = $1`, depID, req.Reason); err != nil {
		serverErr(w)
		return
	}
	// el cliente recibe otro plazo completo para enviar un comprobante correcto
	if _, err := tx.Exec(ctx, `UPDATE appointments SET hold_expires_at = $2 WHERE id = $1`, id, a.now().Add(time.Duration(st.TransferHoldHours)*time.Hour)); err != nil {
		serverErr(w)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	a.logActivity(ctx, p.UserID, p.BusinessID, "transfer_rejected", map[string]any{"appointment_id": id})
	w.WriteHeader(http.StatusNoContent)
}

// transferRefunded es POST /api/appointments/{id}/transfer/refunded: el negocio ya regresó el anticipo por transferencia.
func (a *API) transferRefunded(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	tag, err := a.DB.Exec(r.Context(),
		`UPDATE deposits SET status = 'refunded', refunded_at = $3, refund_due = false
		  WHERE appointment_id = $1 AND business_id = $2 AND method = 'transfer' AND status = 'paid' AND refund_due`, id, p.BusinessID, a.now())
	if err != nil {
		serverErr(w)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusConflict, "no hay una devolución pendiente en esta cita")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getTransferInfo es GET /api/transfer-info: los datos bancarios que el personal enseña a quien va a transferir.
func (a *API) getTransferInfo(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	st, err := loadBookingSettings(r.Context(), a.DB, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": st.TransferEnabled, "bank": st.TransferBank, "holder": st.TransferHolder, "clabe": st.TransferClabe,
		"whatsapp": waDigits(st.TransferWhatsApp), "deposit_percent": st.DepositPercent, "hold_hours": st.TransferHoldHours,
	})
}
