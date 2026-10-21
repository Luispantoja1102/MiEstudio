package api

// mailEnabled dice si hay un servidor de correo configurado (se completa con el módulo de correo).
func (a *API) mailEnabled() bool { return false }
