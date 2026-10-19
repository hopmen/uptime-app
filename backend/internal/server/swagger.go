package server

import (
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger/v2"

	"uptime-app/backend/docs"
)

func registerSwagger(mux *http.ServeMux) {
	mux.HandleFunc("GET /swagger/{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
	})
	contracts := http.StripPrefix("/swagger/", http.FileServer(http.FS(docs.Files)))
	mux.Handle("GET /swagger/openapi.json", contracts)
	mux.Handle("GET /swagger/openapi.yaml", contracts)
	// Keep the conventional doc.json URL consistent with the contract shown in UI.
	mux.HandleFunc("GET /swagger/doc.json", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/swagger/openapi.json"
		contracts.ServeHTTP(w, r)
	})
	mux.HandleFunc("GET /swagger/", httpSwagger.Handler(httpSwagger.URL("openapi.json")))
}
