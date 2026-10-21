package api

import (
	"fmt"
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

// cents da formato de pesos a una cantidad en centavos: 15050 -> "150.50".
func cents(n int) string {
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	return fmt.Sprintf("%s%d.%02d", sign, n/100, n%100)
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
