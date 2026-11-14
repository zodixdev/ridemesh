package main

import (
	"context"
	"fmt"

	"github.com/zodixdev/ridemesh/internal/accounts"
	"github.com/zodixdev/ridemesh/internal/gateway/handler"
	"github.com/zodixdev/ridemesh/internal/gateway/ratelimit"
	"github.com/zodixdev/ridemesh/internal/gateway/realtime"
	"github.com/zodixdev/ridemesh/pkg/authn"
	"github.com/zodixdev/ridemesh/pkg/config"
	"github.com/zodixdev/ridemesh/pkg/httpapi"
	"github.com/zodixdev/ridemesh/pkg/messaging"
	"github.com/zodixdev/ridemesh/pkg/mongox"
)

func main() { httpapi.Main(run) }

func run(ctx context.Context) error {
	trip, err := httpapi.Upstream(config.GetString("TRIP_URL", "http://localhost:8082"))
	if err != nil {
		return fmt.Errorf("TRIP_URL: %w", err)
	}
	driver, err := httpapi.Upstream(config.GetString("DRIVER_URL", "http://localhost:8083"))
	if err != nil {
		return fmt.Errorf("DRIVER_URL: %w", err)
	}
	payment, err := httpapi.Upstream(config.GetString("PAYMENT_URL", "http://localhost:8084"))
	if err != nil {
		return fmt.Errorf("PAYMENT_URL: %w", err)
	}
	delegation, err := authn.LoadDelegation()
	if err != nil {
		return err
	}
	database, err := mongox.Open(ctx)
	if err != nil {
		return err
	}
	defer mongox.Close(database.Client())
	store := accounts.NewStore(database)
	if err := store.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("prepare account indexes: %w", err)
	}
	auth, err := accounts.NewService(store)
	if err != nil {
		return err
	}
	if err := auth.Bootstrap(ctx, config.GetString("RIDEMESH_BOOTSTRAP_EMAIL", ""), config.GetString("RIDEMESH_BOOTSTRAP_PASSWORD", "")); err != nil {
		return err
	}
	limitConfig, err := ratelimit.LoadConfig()
	if err != nil {
		return fmt.Errorf("rate limit configuration: %w", err)
	}
	limitBackend, err := ratelimit.OpenRedis(limitConfig.RedisURL)
	if err != nil {
		return err
	}
	defer limitBackend.Close()
	limits, err := ratelimit.New(limitBackend, limitConfig)
	if err != nil {
		return fmt.Errorf("initialize rate limiter: %w", err)
	}
	streamConfig, err := realtime.LoadConfig()
	if err != nil {
		return fmt.Errorf("real-time stream configuration: %w", err)
	}
	hub := realtime.NewHub(streamConfig)
	streams := realtime.NewHTTP(hub, trip, delegation, streamConfig)
	worker, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()
	go messaging.RunRideConsumer(worker, config.RabbitURL(), hub.Resync, hub.Publish)
	ready := func(ctx context.Context) error { return database.Client().Ping(ctx, nil) }
	return httpapi.Run(ctx, config.GetString("GATEWAY_PORT", "8080"),
		handler.NewWithRealtime(trip, driver, payment, auth, delegation, limits, ready, streams))
}
