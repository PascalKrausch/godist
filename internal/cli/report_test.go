package cli

import (
	"testing"

	"github.com/PascalKrausch/godist/pkg/distribution"
)

func TestGetDistributionNameZeroInflated(t *testing.T) {
	dist := distribution.ZeroInflated{
		Base:  distribution.Gamma{Alpha: 2, Beta: 1},
		PZero: 0.3,
	}
	if got := getDistributionName(dist); got != "ZI-Gamma" {
		t.Fatalf("getDistributionName = %q, want %q", got, "ZI-Gamma")
	}
}
