package api

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/wyw14/cry-126/internal/service"
)

type Server struct {
	runtime *service.Runtime
	web     fs.FS
}

func NewServer(runtime *service.Runtime, web fs.FS) *Server {
	return &Server{runtime: runtime, web: web}
}

func (server *Server) Router() http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(15 * time.Second))
	router.Get("/healthz", server.health)
	router.Route("/api", func(router chi.Router) {
		router.Get("/operations", server.operations)
		router.Post("/operations", server.startOperation)
		router.Get("/equipment", server.equipment)
		router.Post("/equipment/drain", server.startDrain)
		router.Post("/equipment/levels", server.updateLevel)
		router.Post("/equipment/gates/reverse", server.reverseGate)
		router.Post("/equipment/pumps/stop", server.stopPump)
		router.Get("/interlocks", server.interlocks)
		router.Post("/interlocks/close", server.emergencyClose)
		router.Post("/interlocks/release", server.releaseClosure)
		router.Get("/incidents", server.incidents)
		router.Get("/forecast", server.forecast)
		router.Post("/forecast", server.updateForecast)
		router.Post("/telemetry", server.receiveTelemetry)
		router.Get("/status", server.status)
	})
	if server.web != nil {
		files := http.FileServer(http.FS(server.web))
		router.Get("/", func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(writer, request, "/operations.html", http.StatusTemporaryRedirect)
		})
		router.Handle("/*", files)
	}
	return router
}
