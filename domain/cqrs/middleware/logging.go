// Package middleware 提供 CQRS 命令和查询处理器的通用中间件.
package middleware

import (
	"context"
	"time"

	"github.com/Tsukikage7/servex/v2/domain/cqrs"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// CommandLogging 为命令处理器添加日志装饰器.
// 记录命令名称、执行耗时和错误信息.
func CommandLogging[C, R any](log logger.Logger, commandName string) cqrs.CommandMiddleware[C, R] {
	return func(next cqrs.CommandHandler[C, R]) cqrs.CommandHandler[C, R] {
		return &commandLoggingHandler[C, R]{
			next:        next,
			logger:      log,
			commandName: commandName,
		}
	}
}

type commandLoggingHandler[C, R any] struct {
	next        cqrs.CommandHandler[C, R]
	logger      logger.Logger
	commandName string
}

func (h *commandLoggingHandler[C, R]) Handle(ctx context.Context, cmd C) (C, R, error) {
	start := time.Now()
	c, r, err := h.next.Handle(ctx, cmd)
	elapsed := time.Since(start)

	if err != nil {
		h.logger.With(
			logger.String("command", h.commandName),
			logger.Duration("elapsed", elapsed),
			logger.Err(err),
		).Error("[CQRS] 命令执行失败")
	} else {
		h.logger.With(
			logger.String("command", h.commandName),
			logger.Duration("elapsed", elapsed),
		).Debug("[CQRS] 命令执行成功")
	}

	return c, r, err
}

// QueryLogging 为查询处理器添加日志装饰器.
// 记录查询名称、执行耗时和错误信息.
func QueryLogging[Q, R any](log logger.Logger, queryName string) cqrs.QueryMiddleware[Q, R] {
	return func(next cqrs.QueryHandler[Q, R]) cqrs.QueryHandler[Q, R] {
		return &queryLoggingHandler[Q, R]{
			next:      next,
			logger:    log,
			queryName: queryName,
		}
	}
}

type queryLoggingHandler[Q, R any] struct {
	next      cqrs.QueryHandler[Q, R]
	logger    logger.Logger
	queryName string
}

func (h *queryLoggingHandler[Q, R]) Handle(ctx context.Context, query Q) (R, error) {
	start := time.Now()
	r, err := h.next.Handle(ctx, query)
	elapsed := time.Since(start)

	if err != nil {
		h.logger.With(
			logger.String("query", h.queryName),
			logger.Duration("elapsed", elapsed),
			logger.Err(err),
		).Error("[CQRS] 查询执行失败")
	} else {
		h.logger.With(
			logger.String("query", h.queryName),
			logger.Duration("elapsed", elapsed),
		).Debug("[CQRS] 查询执行成功")
	}

	return r, err
}
