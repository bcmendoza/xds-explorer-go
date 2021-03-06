package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bcmendoza/xds-explorer/handlers"
	"github.com/bcmendoza/xds-explorer/model"
	"github.com/bcmendoza/xds-explorer/stream"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func main() {
	var err error

	// startup
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.Stamp}).
		With().Timestamp().Str("service", "xds-explorer").Logger()
	logger.Info().Msg("Startup XDS Explorer")

	// signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	logger.Info().Msg("Watch OS signals")

	// context
	ctx, cancelFunc := context.WithCancel(context.Background())

	// request channel
	requestChan := make(chan model.Request, 1)

	// model
	xdsData := model.New()

	// GRPC stream
	streamLogger := logger.With().Str("package", "stream").Logger()
	viper.AutomaticEnv()
	viper.SetDefault("XDS_SERVER", "localhost:50000")
	xdsServer := viper.GetString("XDS_SERVER")
	go stream.Listen(xdsServer, ctx, requestChan, xdsData, streamLogger)

	// REST server
	serverLogger := logger.With().Str("package", "handlers").Logger()
	viper.SetDefault("CLIENT_PORT", "3001")
	server := http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%s", viper.GetString("CLIENT_PORT")),
		Handler: handlers.Handlers(requestChan, xdsData, serverLogger),
	}
	go func() {
		serverLogger.Info().Msg("Startup REST server")
		if err = server.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
			serverLogger.Error().AnErr("server.ListenAndServe()", err).Msg("REST server error")
		}
	}()

	// shutdown
	s := <-sigChan
	cancelFunc()
	if err = server.Close(); err != nil {
		logger.Error().AnErr("server.Close()", err).Msg("REST server shutdown error")
	} else {
		logger.Info().Msg("Shutdown REST server")
	}
	logger.Info().Str("signal", s.String()).Msg("Shutdown XDS Explorer")
}
