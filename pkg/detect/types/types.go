package types

import (
	"context"

	"github.com/cvelab/vuls/pkg/types"
)

type Detector interface {
	Name() string
	Detect(context.Context, *types.Host) error
}
