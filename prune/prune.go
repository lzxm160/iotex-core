// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
)

var (
	// ErrInternalServer indicates the internal server error
	ErrInternalServer = errors.New("internal server error")
)

type (
	// Pruner is the interface for state Pruner
	Pruner interface {
		Start(context.Context) error
		Stop(context.Context) error
	}
)

// Prune provides api for user to query blockchain data
type Prune struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    config.Config
}

// NewPrune creates a new server
func NewPrune(cfg config.Config) Pruner {
	return &Prune{
		cfg: cfg,
	}
}

// Start starts the API server
func (p *Prune) Start(ctx context.Context) error {
	log.L().Info("Prune server is running.")
	p.ctx, p.cancel = context.WithCancel(ctx)
	go func() {
		if err := p.start(); err != nil {
			log.L().Fatal("Node failed to serve.", zap.Error(err))
		}
	}()
	return nil
}

// Stop stops the API server
func (p *Prune) Stop(ctx context.Context) error {
	p.cancel()
	return nil
}

func (p *Prune) start() error {
	d := time.Duration(10) * time.Second
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			log.L().Info("start run :")
		case <-p.ctx.Done():
			log.L().Info("exit :")
			return nil
		}

	}

	return nil
}
