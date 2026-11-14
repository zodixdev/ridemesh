package main

import (
	"context"
	"fmt"

	"github.com/zodixdev/ridemesh/internal/payment/handler"
	"github.com/zodixdev/ridemesh/internal/payment/repository"
	"github.com/zodixdev/ridemesh/pkg/authn"
	"github.com/zodixdev/ridemesh/pkg/config"
	"github.com/zodixdev/ridemesh/pkg/httpapi"
	"github.com/zodixdev/ridemesh/pkg/messaging"
	"github.com/zodixdev/ridemesh/pkg/mongox"
)

func main() { httpapi.Main(run) }

func run(ctx context.Context) error {
	delegation, err := authn.LoadDelegation()
	if err != nil {
		return err
	}
	tripURL, err := httpapi.Upstream(config.GetString("TRIP_URL", "http://localhost:8082"))
	if err != nil {
		return fmt.Errorf("TRIP_URL: %w", err)
	}
	database, err := mongox.Open(ctx)
	if err != nil {
		return err
	}
	defer mongox.Close(database.Client())
	repo := repository.New(database)
	worker, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		messaging.RunConsumer(worker, config.RabbitURL(), repo.Record)
	}()
	defer func() {
		cancel()
		<-done
	}()
	mux := httpapi.Mux("payment", func(ctx context.Context) error { return database.Client().Ping(ctx, nil) })
	handler.Register(mux, repo, tripURL, delegation)
	return httpapi.Run(ctx, config.GetString("PAYMENT_HTTP_PORT", "8084"), delegation.Protect("payment", mux))
}
