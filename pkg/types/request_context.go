package types

import "context"

type ctxKeyRequestedQuant struct{}

type ctxKeyRequestedModel struct{}

// WithRequestedModel returns a context carrying the model name of a
// model-centric `models/<name>:<quant>` serve (e.g. "qnib/minicpm5"). It is
// threaded to the manipulation engine so that composing a model pull selects
// ONLY that model's `model/<name>` + `config/<name>` manipulations — a profile
// may host many models, and pulling model A must never inject model B.
func WithRequestedModel(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestedModel{}, name)
}

// RequestedModelFromContext returns the requested model name, or "" when the
// serve path did not set one — in which case the manipulation engine applies no
// model scoping (preserving the pre-model-centric, single-model-per-profile
// behaviour).
func RequestedModelFromContext(ctx context.Context) string {
	if n, ok := ctx.Value(ctxKeyRequestedModel{}).(string); ok {
		return n
	}
	return ""
}

// WithRequestedQuant returns a context carrying the explicitly-requested model
// quant — the pull tag of a model-centric `models/<name>:<quant>` serve. It is
// threaded to the manipulation engine so an explicit tag pins that quant
// (fit-check/reject) while an empty value means best-fit (the default).
func WithRequestedQuant(ctx context.Context, quant string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestedQuant{}, quant)
}

// RequestedQuantFromContext returns the explicitly-requested quant, or "" (best-fit)
// when the serve path did not set one — preserving the pre-model-centric behaviour.
func RequestedQuantFromContext(ctx context.Context) string {
	if q, ok := ctx.Value(ctxKeyRequestedQuant{}).(string); ok {
		return q
	}
	return ""
}
