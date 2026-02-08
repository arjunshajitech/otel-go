package main

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func setupOTelSDK(ctx context.Context) (func(context.Context) error, error) {
	var shutdowns []func(context.Context) error

	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdowns {
			err = errors.Join(err, fn(ctx))
		}
		shutdowns = nil
		return err
	}

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	// ---- Traces ----
	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithInsecure())
	if err != nil {
		panic(err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(
			traceExporter,
			sdktrace.WithBatchTimeout(2*time.Second),
		),
	)
	otel.SetTracerProvider(tp)
	shutdowns = append(shutdowns, tp.Shutdown)

	// ---- Metrics ----
	metricExporter, err := otlpmetrichttp.New(
		ctx,
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return shutdown, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				metricExporter,
				sdkmetric.WithInterval(2*time.Second),
			),
		),
	)
	otel.SetMeterProvider(mp)
	shutdowns = append(shutdowns, mp.Shutdown)

	// ---- Logs ----
	logExporter, err := otlploghttp.New(ctx, otlploghttp.WithInsecure())
	if err != nil {
		panic(err)
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(
			sdklog.NewBatchProcessor(logExporter),
		),
	)
	global.SetLoggerProvider(lp)
	shutdowns = append(shutdowns, lp.Shutdown)

	return shutdown, nil
}
