package api

import (
	"net/http"
	"time"
)

// NoticeVersion nombra el texto vigente de /privacidad y /terminos. Cámbiala cuando cualquiera de los dos cambie
// de fondo: desde entonces las nuevas aceptaciones se registran con ella.
const NoticeVersion = "2026-10"

// ClientNoticeVersion es la del aviso que ven los clientes de un negocio al reservar.
const ClientNoticeVersion = "2026-10"

// Operator son los datos de quien opera la plataforma, que el aviso de privacidad debe mostrar.
type Operator struct {
	Name    string // LEGAL_NAME: razón social o nombre del responsable (por omisión, Hyperion)
	Address string // LEGAL_ADDRESS: domicilio (opcional)
	Email   string // LEGAL_EMAIL: correo de contacto para solicitudes de privacidad y dudas
}

func (a *API) legalInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version": NoticeVersion, "client_notice_version": ClientNoticeVersion,
		"operator": map[string]string{"name": a.Operator.Name, "address": a.Operator.Address, "email": a.Operator.Email},
		"updated":  NoticeVersion + "-01",
		"year":     time.Now().Year(),
	})
}
