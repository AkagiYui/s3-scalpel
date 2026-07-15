package s3x

import "testing"

func TestPlanParts(t *testing.T) {
	// Exact multiple of MinPartSize -> two full parts.
	plans := planParts(2*MinPartSize, MinPartSize)
	if len(plans) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(plans))
	}
	if plans[0].Number != 1 || plans[0].Offset != 0 || plans[0].Length != MinPartSize {
		t.Errorf("part0=%+v", plans[0])
	}
	if plans[1].Number != 2 || plans[1].Offset != MinPartSize || plans[1].Length != MinPartSize {
		t.Errorf("part1=%+v", plans[1])
	}

	// Remainder in the final part.
	plans = planParts(MinPartSize+123, MinPartSize)
	if len(plans) != 2 || plans[1].Length != 123 {
		t.Fatalf("remainder plan=%+v", plans)
	}

	// Coverage is contiguous and complete.
	var total int64
	for i, p := range plans {
		if p.Number != int32(i+1) {
			t.Errorf("part %d has number %d", i, p.Number)
		}
		total += p.Length
	}
	if total != MinPartSize+123 {
		t.Errorf("covered %d bytes, want %d", total, MinPartSize+123)
	}

	// partSize below the minimum is clamped up.
	plans = planParts(MinPartSize, 1024)
	if len(plans) != 1 || plans[0].Length != MinPartSize {
		t.Errorf("clamp plan=%+v", plans)
	}

	// Zero-byte object still yields a single empty part.
	plans = planParts(0, MinPartSize)
	if len(plans) != 1 || plans[0].Length != 0 {
		t.Errorf("zero plan=%+v", plans)
	}
}

func TestClampConcurrency(t *testing.T) {
	cases := []struct{ n, parts, want int }{
		{0, 10, DefaultPartConcurrency},
		{8, 3, 3},
		{2, 10, 2},
		{-5, 4, DefaultPartConcurrency},
		{100, 1, 1},
	}
	for _, c := range cases {
		if got := clampConcurrency(c.n, c.parts); got != c.want {
			t.Errorf("clampConcurrency(%d,%d)=%d want %d", c.n, c.parts, got, c.want)
		}
	}
}
