package server_test

import (
	"strings"
	"testing"
)

// S59: whiteboards in a conversation — members draw together; concurrent moves of one shape resolve to one
// state; others see changes and cursors live; outsiders see nothing.
func TestWhiteboard(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		eve := anon.signedIn("eve@x.uz")
		_, devMe := dev.do("GET", "/api/v1/me", nil)
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "arch", "private": true})
		conv := "/api/v1/conversations/" + ch["id"].(string)
		lead.do("POST", conv+"/members", map[string]string{"user_id": devMe["id"].(string)})

		code, b := lead.do("POST", conv+"/whiteboards", map[string]string{"title": "Architecture"})
		expect(t, code, 201, b)
		code, x := eve.do("POST", conv+"/whiteboards", map[string]string{"title": "x"})
		expect(t, code, 404, x)
		_, list := dev.do("GET", conv+"/whiteboards", nil)
		if items := list["items"].([]any); len(items) != 1 || items[0].(map[string]any)["title"] != "Architecture" {
			t.Fatalf("list = %v", list)
		}
		url := "/api/v1/whiteboards/" + b["id"].(string)
		devStream := dev.stream("")

		box := map[string]any{"id": "api", "kind": "rect", "x": 10, "y": 20, "w": 120, "h": 60, "text": "API", "color": "#0b6fd8", "points": []any{}, "version": 0,
			"stroke": 3, "fill": true, "dashed": true, "z": 2} // S91 styles, S93 order
		code, r := lead.do("POST", url+"/ops", map[string]any{"ops": []any{map[string]any{"op": "add", "element": box}}})
		expect(t, code, 200, r)
		res := r["results"].([]any)[0].(map[string]any)
		if el := res["element"].(map[string]any); res["applied"] != true || el["version"] != float64(1) || el["stroke"] != float64(3) || el["fill"] != true || el["dashed"] != true || el["z"] != float64(2) {
			t.Fatalf("add = %v", r)
		}
		if e := next(t, devStream, "whiteboard.ops"); !strings.Contains(e.data, `"text":"API"`) {
			t.Fatalf("ops event = %+v", e)
		}

		// Both move the box from version 1: the first wins, the second learns the current position.
		moveA := map[string]any{"id": "api", "kind": "rect", "x": 200, "y": 20, "w": 120, "h": 60, "text": "API", "color": "#0b6fd8", "points": []any{}, "version": 1}
		moveB := map[string]any{"id": "api", "kind": "rect", "x": 300, "y": 20, "w": 120, "h": 60, "text": "API", "color": "#0b6fd8", "points": []any{}, "version": 1}
		lead.do("POST", url+"/ops", map[string]any{"ops": []any{map[string]any{"op": "update", "element": moveA}}})
		_, r = dev.do("POST", url+"/ops", map[string]any{"ops": []any{map[string]any{"op": "update", "element": moveB}}})
		res = r["results"].([]any)[0].(map[string]any)
		if res["applied"] != false || res["error"] != "stale" || res["element"].(map[string]any)["x"] != float64(200) {
			t.Fatalf("losing move = %v", r)
		}
		code, r = dev.do("POST", url+"/ops", map[string]any{"ops": []any{map[string]any{"op": "add", "element": map[string]any{"id": "bad", "kind": "triangle", "points": []any{}}}}})
		expect(t, code, 200, r)
		if res := r["results"].([]any)[0].(map[string]any); res["applied"] != false || res["error"] != "invalid" {
			t.Fatalf("invalid = %v", r)
		}

		_, board := dev.do("GET", url, nil)
		if els := board["elements"].([]any); len(els) != 1 || els[0].(map[string]any)["x"] != float64(200) {
			t.Fatalf("board = %v", board)
		}
		code, x = eve.do("GET", url, nil)
		expect(t, code, 404, x)

		code, x = lead.do("POST", url+"/cursor", map[string]float64{"x": 5, "y": 6})
		expect(t, code, 204, x)
		if e := next(t, devStream, "whiteboard.cursor"); !strings.Contains(e.data, `"x":5`) {
			t.Fatalf("cursor = %+v", e)
		}
	})
}

// S86: comments pinned to elements; @mentions notify people who may read the conversation.
func TestWhiteboardComments(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		eve := anon.signedIn("eve@x.uz")
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "design"})
		conv := ch["id"].(string)
		dev.do("POST", "/api/v1/conversations/"+conv+"/join", nil)
		_, b := lead.do("POST", "/api/v1/conversations/"+conv+"/whiteboards", map[string]string{"title": "Architecture"})
		url := "/api/v1/whiteboards/" + b["id"].(string) + "/comments"
		code, c := lead.do("POST", url, map[string]string{"element_id": "e-1", "body": "@dev@x.uz is this cached?"})
		expect(t, code, 201, c)
		code, x := eve.do("POST", url, map[string]string{"element_id": "e-1", "body": "hi"})
		expect(t, code, 404, x)
		if _, l := dev.do("GET", url, nil); len(l["items"].([]any)) != 1 {
			t.Fatalf("comments = %v", l)
		}
		eventuallyJSON(t, dev, "/api/v1/notifications", func(n map[string]any) bool { return len(n["items"].([]any)) == 1 })
		code, x = dev.do("DELETE", "/api/v1/whiteboard-comments/"+c["id"].(string), nil)
		expect(t, code, 403, x)
		code, x = lead.do("DELETE", "/api/v1/whiteboard-comments/"+c["id"].(string), nil)
		expect(t, code, 204, x)
	})
}
