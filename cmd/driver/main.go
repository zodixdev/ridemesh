package main

import (
	"context"
	"fmt"

	"github.com/zodixdev/ridemesh/internal/driver/handler"
	"github.com/zodixdev/ridemesh/internal/driver/repository"
	"github.com/zodixdev/ridemesh/internal/driver/service"
	"github.com/zodixdev/ridemesh/pkg/authn"
	"github.com/zodixdev/ridemesh/pkg/config"
	"github.com/zodixdev/ridemesh/pkg/httpapi"
	"github.com/zodixdev/ridemesh/pkg/mongox"
)

func main() { httpapi.Main(run) }

func run(ctx context.Context) error {
	delegation, err := authn.LoadDelegation()
	if err != nil {
		return err
	}
	database, err := mongox.Open(ctx)
	if err != nil {
		return err
	}
	defer mongox.Close(database.Client())
	repo := repository.NewMongoRepository(database)
	if err := repo.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("prepare driver indexes: %w", err)
	}
	mux := httpapi.Mux("driver", func(ctx context.Context) error { return database.Client().Ping(ctx, nil) })
	handler.Register(mux, service.New(repo))
	return httpapi.Run(ctx, config.GetString("DRIVER_HTTP_PORT", "8083"), delegation.Protect("driver", mux))
}
