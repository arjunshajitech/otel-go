package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const name = "dice-app"

var (
	tracer  trace.Tracer
	meter   metric.Meter
	logger  *slog.Logger
	rollCnt metric.Int64Counter
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	otelShutdown, err := setup(ctx)
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	count := 0

	for range ticker.C {
		fmt.Println("############# Running #############", time.Now())

		switch count {

		case 2:
			fmt.Println("---- SHUTDOWN OTel ----")
			_ = otelShutdown(context.Background())
			stop()
			count++

		case 20:
			fmt.Println("---- RESTART OTel ----")
			ctx, stop = signal.NotifyContext(context.Background(), os.Interrupt)
			otelShutdown, err = setup(ctx)
			if err != nil {
				panic(err)
			}
			count = 0

		default:
			if count < 2 {
				incrementCounter(ctx)
			}
			count++
		}
	}
}

func incrementCounter(ctx context.Context) {
	ctx, span := tracer.Start(ctx, "roll")
	defer span.End()

	attr := attribute.Int("roll.value", 8)
	span.SetAttributes(attr)
	rollCnt.Add(ctx, 1, metric.WithAttributes(attr))

	logger.InfoContext(ctx, "dice rolled", "value", 8)
}

func setup(ctx context.Context) (func(context.Context) error, error) {
	shutdown, err := setupOTelSDK(ctx)
	if err != nil {
		return nil, err
	}

	// IMPORTANT: bind AFTER providers are set
	tracer = otel.Tracer(name)
	meter = otel.Meter(name)
	logger = otelslog.NewLogger(name)

	rollCnt, err = meter.Int64Counter(
		"dice.rolls",
		metric.WithDescription("Number of dice rolls"),
		metric.WithUnit("{roll}"),
	)
	if err != nil {
		_ = shutdown(ctx)
		return nil, err
	}

	return shutdown, nil
}
