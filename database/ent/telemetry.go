package ent

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const telemetryName = "github.com/liujitcn/kratos-kit/database/ent"

// telemetryDriver 为 Ent driver 增加追踪和指标采集能力。
type telemetryDriver struct {
	dialect.Driver
	dialectName  string
	enableTrace  bool
	enableMetric bool
	tracer       trace.Tracer
	metrics      *driverMetrics
}

// driverMetrics 保存数据库操作指标。
type driverMetrics struct {
	operationCount    metric.Int64Counter
	operationDuration metric.Float64Histogram
}

// telemetryTx 为 Ent 事务增加追踪和指标采集能力。
type telemetryTx struct {
	dialect.Tx
	ctx    context.Context
	parent *telemetryDriver
}

// wrapTelemetry 按配置包装 Ent driver。
func wrapTelemetry(drv dialect.Driver, dialectName string, enableTrace bool, enableMetric bool) (dialect.Driver, error) {
	if !enableTrace && !enableMetric {
		return drv, nil
	}

	wrapped := &telemetryDriver{
		Driver:       drv,
		dialectName:  dialectName,
		enableTrace:  enableTrace,
		enableMetric: enableMetric,
	}
	if enableTrace {
		wrapped.tracer = otel.Tracer(telemetryName)
	}
	if enableMetric {
		meter := otel.Meter(telemetryName)
		operationCount, err := meter.Int64Counter("db.client.operations")
		if err != nil {
			return nil, err
		}
		var operationDuration metric.Float64Histogram
		operationDuration, err = meter.Float64Histogram("db.client.operation.duration")
		if err != nil {
			return nil, err
		}
		wrapped.metrics = &driverMetrics{
			operationCount:    operationCount,
			operationDuration: operationDuration,
		}
	}
	return wrapped, nil
}

// Exec 执行带追踪和指标的 Ent Exec 操作。
func (d *telemetryDriver) Exec(ctx context.Context, query string, args, v any) error {
	return d.observe(ctx, "exec", query, func(ctx context.Context) error {
		return d.Driver.Exec(ctx, query, args, v)
	})
}

// Query 执行带追踪和指标的 Ent Query 操作。
func (d *telemetryDriver) Query(ctx context.Context, query string, args, v any) error {
	return d.observe(ctx, "query", query, func(ctx context.Context) error {
		return d.Driver.Query(ctx, query, args, v)
	})
}

// Tx 开启带追踪和指标的 Ent 事务。
func (d *telemetryDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	var tx dialect.Tx
	err := d.observe(ctx, "tx", "", func(ctx context.Context) error {
		var txErr error
		tx, txErr = d.Driver.Tx(ctx)
		return txErr
	})
	if err != nil {
		return nil, err
	}
	return &telemetryTx{
		Tx:     tx,
		ctx:    ctx,
		parent: d,
	}, nil
}

// observe 统一记录数据库操作追踪和指标。
func (d *telemetryDriver) observe(ctx context.Context, operation string, query string, fn func(context.Context) error) error {
	start := time.Now()
	attrs := []attribute.KeyValue{
		attribute.String("db.system", d.dialectName),
		attribute.String("db.operation", operation),
	}
	if query != "" {
		attrs = append(attrs, attribute.String("db.statement", query))
	}

	if d.enableTrace {
		var span trace.Span
		ctx, span = d.tracer.Start(ctx, "ent."+operation, trace.WithAttributes(attrs...))
		defer span.End()
		err := fn(ctx)
		d.recordMetrics(ctx, operation, start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
		span.SetStatus(codes.Ok, "")
		return nil
	}

	err := fn(ctx)
	d.recordMetrics(ctx, operation, start, err)
	return err
}

// recordMetrics 记录数据库操作次数和耗时。
func (d *telemetryDriver) recordMetrics(ctx context.Context, operation string, start time.Time, err error) {
	if !d.enableMetric || d.metrics == nil {
		return
	}
	status := "ok"
	if err != nil {
		status = "error"
	}
	attrs := metric.WithAttributes(
		attribute.String("db.system", d.dialectName),
		attribute.String("db.operation", operation),
		attribute.String("db.status", status),
	)
	d.metrics.operationCount.Add(ctx, 1, attrs)
	d.metrics.operationDuration.Record(ctx, float64(time.Since(start).Milliseconds()), attrs)
}

// Exec 执行带追踪和指标的事务 Exec 操作。
func (t *telemetryTx) Exec(ctx context.Context, query string, args, v any) error {
	return t.parent.observe(ctx, "tx.exec", query, func(ctx context.Context) error {
		return t.Tx.Exec(ctx, query, args, v)
	})
}

// Query 执行带追踪和指标的事务 Query 操作。
func (t *telemetryTx) Query(ctx context.Context, query string, args, v any) error {
	return t.parent.observe(ctx, "tx.query", query, func(ctx context.Context) error {
		return t.Tx.Query(ctx, query, args, v)
	})
}

// Commit 提交带追踪和指标的事务。
func (t *telemetryTx) Commit() error {
	err := t.parent.observe(t.ctx, "tx.commit", "", func(context.Context) error {
		return t.Tx.Commit()
	})
	if err != nil {
		return fmt.Errorf("ent tx commit: %w", err)
	}
	return nil
}

// Rollback 回滚带追踪和指标的事务。
func (t *telemetryTx) Rollback() error {
	err := t.parent.observe(t.ctx, "tx.rollback", "", func(context.Context) error {
		return t.Tx.Rollback()
	})
	if err != nil {
		return fmt.Errorf("ent tx rollback: %w", err)
	}
	return nil
}

var _ dialect.Driver = (*telemetryDriver)(nil)
var _ dialect.Tx = (*telemetryTx)(nil)
