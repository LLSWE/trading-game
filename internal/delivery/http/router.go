package http

import (
	"context"
	"log"
	"net/http"

	"github.com/LLSWE/trading-game/internal/config"
	"github.com/go-chi/chi/v5"
	"go.uber.org/fx"
)

func NewRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"UP"}`))
	})

	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware)
	})

	return r
}

func RegisterServer(lc fx.Lifecycle, r *chi.Mux, cfg *config.Config) {
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Printf("Running Server on http://localhost:%s", cfg.Port)
			go func() {
				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Printf("Erro on http server: %v\n", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Println("Graceful shutdown...")
			return server.Shutdown(ctx)
		},
	})
}
