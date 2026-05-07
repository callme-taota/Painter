package application

import "testing"

func TestOverviewService(t *testing.T) {
	svc := NewOverviewService()
	data := svc.GetOverview()
	if data.QPS <= 0 {
		t.Fatal("expected positive qps")
	}
}
