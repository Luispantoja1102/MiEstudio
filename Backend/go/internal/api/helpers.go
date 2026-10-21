package api

import (
	"net/http"
	"strconv"
)

// pathID lee {id} de la ruta; responde 400 si no es un entero positivo.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "id inválido")
		return 0, false
	}
	return id, true
}

func serverErr(w http.ResponseWriter) { writeErr(w, http.StatusInternalServerError, "error interno") }
