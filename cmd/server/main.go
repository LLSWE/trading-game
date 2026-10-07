package main

import (
	"github.com/LLSWE/trading-game/internal/config"
	"github.com/LLSWE/trading-game/internal/delivery/http"
	"github.com/LLSWE/trading-game/internal/delivery/worker"
	"github.com/LLSWE/trading-game/internal/repository"
	"github.com/LLSWE/trading-game/internal/usecase"
	"go.uber.org/fx"
)

func main() {
	app := fx.New(

		fx.Provide(
			config.Load,
			repository.NewDatabasePool,
			repository.NewWalletRepository,
			repository.NewWageringRepository,
			usecase.NewWageringUseCase,
			http.NewHandler,
			http.NewRouter,
			worker.NewOutboxDispatcher,
			worker.NewSQSConsumer,
		),

		fx.Invoke(http.RegisterServer, worker.RegisterOutboxWorker, worker.RegisterSQSConsumerWorker),
	)

	app.Run()
}
