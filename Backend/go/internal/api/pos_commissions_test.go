package api

import "testing"

func i64(n int64) *int64 { return &n }

func TestPickPercent(t *testing.T) {
	marco, ana := i64(1), i64(2)
	corte, barba, cera := i64(10), i64(11), i64(20)
	rules := []commissionRule{
		{ServiceID: barba, Percent: 40},                // regla del artículo, para todos
		{UserID: marco, ServiceID: barba, Percent: 55}, // persona + artículo
		{ProductID: cera, Percent: 10},                 // producto
	}
	cases := []struct {
		name             string
		staff, svc, prod *int64
		def, want        float64
	}{
		{"persona+artículo gana", marco, barba, nil, 30, 55},
		{"artículo para otra persona", ana, barba, nil, 30, 40},
		{"sin regla: comisión general en servicios", marco, corte, nil, 30, 30},
		{"producto con regla", ana, nil, cera, 30, 10},
		{"producto sin regla: 0 (la general no aplica)", ana, nil, i64(21), 30, 0},
		{"sin persona: 0", nil, barba, nil, 30, 0},
	}
	for _, c := range cases {
		if got := pickPercent(rules, c.staff, c.svc, c.prod, c.def); got != c.want {
			t.Errorf("%s: esperaba %v, obtuvo %v", c.name, c.want, got)
		}
	}
}

func TestCommissionRounding(t *testing.T) {
	for _, c := range []struct {
		base int
		pct  float64
		want int
	}{
		{15000, 30, 4500}, {15000, 0, 0}, {15000, 100, 15000},
		{1, 50, 1},          // 0.5 centavo → 1
		{3, 50, 2},          // 1.5 → 2
		{999, 33.33, 333},   // 332.9667 → 333
		{12345, 12.5, 1543}, // 1543.125 → 1543
	} {
		if got := commissionOf(c.base, c.pct); got != c.want {
			t.Errorf("%d × %v%%: esperaba %d, obtuvo %d", c.base, c.pct, c.want, got)
		}
	}
}

func TestAllocateDiscountSumsExactly(t *testing.T) {
	cases := []struct {
		parts    []int
		discount int
	}{
		{[]int{10000, 5000, 5000}, 1000}, {[]int{3333, 3333, 3334}, 1}, {[]int{100, 100, 100}, 100},
		{[]int{1}, 1}, {[]int{7, 13, 29, 51}, 33}, {[]int{15000}, 0}, {[]int{5000, 5000}, 10000},
	}
	for _, c := range cases {
		got := allocateDiscount(c.parts, c.discount)
		sum := 0
		for i, g := range got {
			sum += g
			if g < 0 || g > c.parts[i] {
				t.Errorf("%v / %d: reparto fuera de rango en %d: %v", c.parts, c.discount, i, got)
			}
		}
		if sum != c.discount {
			t.Errorf("%v / %d: la suma es %d (%v)", c.parts, c.discount, sum, got)
		}
	}
	// proporcional: 10% a cada partida de 10000 y 5000
	if got := allocateDiscount([]int{10000, 5000}, 1500); got[0] != 1000 || got[1] != 500 {
		t.Errorf("reparto proporcional: %v", got)
	}
}
