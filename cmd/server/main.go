package main

import (
	"github.com/LLSWE/trading-game/internal/config"
	"github.com/LLSWE/trading-game/internal/delivery/http"
	"go.uber.org/fx"
)

func main() {
	app := fx.New(

		fx.Provide(
			config.Load,
			http.NewRouter,
		),

		fx.Invoke(http.RegisterServer),
	)

	app.Run()
}
