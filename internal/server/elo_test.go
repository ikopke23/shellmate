package server

import (
	"testing"
)

func TestCalculate(t *testing.T) {
	tests := []struct {
		name           string
		rA, rB         int
		gamesA, gamesB int
		result         float64
		wantAMin       int
		wantAMax       int
		wantBMin       int
		wantBMax       int
	}{
		{
			name:     "equal ratings white wins K=20",
			rA:       1000, rB: 1000,
			gamesA: 20, gamesB: 20,
			result:   1.0,
			wantAMin: 1008, wantAMax: 1012,
			wantBMin: 988, wantBMax: 992,
		},
		{
			name:     "equal ratings draw no change",
			rA:       1000, rB: 1000,
			gamesA: 20, gamesB: 20,
			result:   0.5,
			wantAMin: 999, wantAMax: 1001,
			wantBMin: 999, wantBMax: 1001,
		},
		{
			name:     "heavy underdog wins large swing",
			rA:       800, rB: 1600,
			gamesA: 20, gamesB: 20,
			result:   1.0,
			wantAMin: 818, wantAMax: 850,
			wantBMin: 1550, wantBMax: 1582,
		},
		{
			name:     "provisional player gamesA<15 uses K=40",
			rA:       1000, rB: 1000,
			gamesA: 5, gamesB: 20,
			result:   1.0,
			wantAMin: 1018, wantAMax: 1022,
			wantBMin: 978, wantBMax: 982,
		},
		{
			name:     "floor check weak player loses stays at 800",
			rA:       800, rB: 2000,
			gamesA: 20, gamesB: 20,
			result:   0.0,
			wantAMin: 800, wantAMax: 800,
			wantBMin: 2000, wantBMax: 2050,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newA, newB := Calculate(tt.rA, tt.rB, tt.gamesA, tt.gamesB, tt.result)
			if newA < tt.wantAMin || newA > tt.wantAMax {
				t.Errorf("newA = %d, want in [%d, %d]", newA, tt.wantAMin, tt.wantAMax)
			}
			if newB < tt.wantBMin || newB > tt.wantBMax {
				t.Errorf("newB = %d, want in [%d, %d]", newB, tt.wantBMin, tt.wantBMax)
			}
		})
	}
}
