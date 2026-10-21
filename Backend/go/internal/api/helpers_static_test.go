package api

import "testing"

func TestStaticCache(t *testing.T) {
	if staticCache("/_app/immutable/chunks/a.js") != "public, max-age=31536000, immutable" {
		t.Fatal("los archivos con huella deben cachearse")
	}
	for _, p := range []string{"/_app/version.json", "/index.html", "/", "/_app/immutable"} {
		if staticCache(p) != "no-cache" {
			t.Fatalf("%s debe revalidarse", p)
		}
	}
}
