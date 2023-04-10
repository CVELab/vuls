package types

import (
	"context"

	"github.com/cvelab/vuls/pkg/types"
)

type Analyzer interface {
	Name() string
	Analyze(context.Context, *AnalyzerHost) error
}

type AnalyzerHost struct {
	Host      *types.Host
	Analyzers []Analyzer
}
