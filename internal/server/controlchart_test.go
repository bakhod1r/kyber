package server_test

import "testing"

// Control chart: each done issue's cycle time (first In Progress → Done) and the averages.
func TestControlChart(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		dev := anon.signedIn("dev@x.uz")
		eve := anon.signedIn("eve@x.uz")
		dev.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		dev.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Ship it", "type": "task"})
		dev.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "in_progress"})
		dev.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "done"})
		cc := eventuallyJSON(t, dev, "/api/v1/projects/KYB/reports/control-chart?days=7", func(b map[string]any) bool {
			return len(b["points"].([]any)) == 1
		})
		p := cc["points"].([]any)[0].(map[string]any)
		if p["key"] != "KYB-1" || p["title"] != "Ship it" || p["done_at"] == nil {
			t.Fatalf("point = %v", p)
		}
		code, cfd := dev.do("GET", "/api/v1/projects/KYB/reports/cumulative-flow?days=3", nil)
		expect(t, code, 200, cfd)
		if days := cfd["days"].([]any); len(days) != 3 || days[2].(map[string]any)["done"] != float64(1) {
			t.Fatalf("cumulative flow = %v", cfd)
		}
		code, b := dev.do("GET", "/api/v1/projects/KYB/reports/cumulative-flow?days=400", nil)
		expect(t, code, 422, b)
		code, b = dev.do("GET", "/api/v1/projects/KYB/reports/control-chart?days=0", nil)
		expect(t, code, 422, b)
		code, b = eve.do("GET", "/api/v1/projects/KYB/reports/control-chart", nil)
		expect(t, code, 404, b)
	})
}
