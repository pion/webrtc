// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"github.com/pion/interceptor/pkg/nack"
	"github.com/pion/interceptor/pkg/report"
	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/interceptor/pkg/twcc"
	"github.com/pion/logging"
)

// interceptorOptions contains options for configuring interceptors.
type interceptorOptions struct {
	loggerFactory logging.LoggerFactory

	nackGeneratorOptions  []nack.GeneratorOption
	nackResponderOptions  []nack.ResponderOption
	reportReceiverOptions []report.ReceiverOption
	reportSenderOptions   []report.SenderOption
	statsOptions          []stats.Option
	twccOptions           []twcc.Option
}

// InterceptorOption is a function that configures InterceptorOptions.
type InterceptorOption func(*interceptorOptions)

// WithInterceptorLoggerFactory sets the logger factory for interceptors.
func WithInterceptorLoggerFactory(loggerFactory logging.LoggerFactory) InterceptorOption {
	return func(o *interceptorOptions) {
		o.loggerFactory = loggerFactory
	}
}

// WithNackGeneratorOptions sets options for the NACK generator interceptor.
func WithNackGeneratorOptions(opts ...nack.GeneratorOption) InterceptorOption {
	return func(o *interceptorOptions) {
		o.nackGeneratorOptions = opts
	}
}

// WithNackResponderOptions sets options for the NACK responder interceptor.
func WithNackResponderOptions(opts ...nack.ResponderOption) InterceptorOption {
	return func(o *interceptorOptions) {
		o.nackResponderOptions = opts
	}
}

// WithReportReceiverOptions sets options for the report receiver interceptor.
func WithReportReceiverOptions(opts ...report.ReceiverOption) InterceptorOption {
	return func(o *interceptorOptions) {
		o.reportReceiverOptions = opts
	}
}

// WithReportSenderOptions sets options for the report sender interceptor.
func WithReportSenderOptions(opts ...report.SenderOption) InterceptorOption {
	return func(o *interceptorOptions) {
		o.reportSenderOptions = opts
	}
}

// WithStatsInterceptorOptions sets options for the stats interceptor.
func WithStatsInterceptorOptions(opts ...stats.Option) InterceptorOption {
	return func(o *interceptorOptions) {
		o.statsOptions = opts
	}
}

// WithTWCCOptions sets options for the TWCC interceptor.
func WithTWCCOptions(opts ...twcc.Option) InterceptorOption {
	return func(o *interceptorOptions) {
		o.twccOptions = opts
	}
}
