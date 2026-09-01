package timeline

import (
	"math"
	"testing"
	"time"
)

func TestFloatMS(t *testing.T) {
	if floatMS(float32(math.NaN())) != 0 || floatMS(float32(math.Inf(1))) != 0 || floatMS(1.5) != 1500*time.Microsecond {
		t.Error("floatMS edge cases are wrong")
	}
}
