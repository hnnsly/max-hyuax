//go:build integration

package httpapi_test

import (
	"strings"
	"testing"
)

// Совет дома целиком: житель предлагает, председатель видит без имени и отвечает, выносит на опрос,
// соседи голосуют по одному разу. Плюс пример данных из миграции 00012.
func TestCouncilFlow(t *testing.T) {
	anna, sergey, nina := login(t, "resident"), login(t, "resident_2"), login(t, "chairman")
	oper, district := login(t, "uk_operator"), login(t, "district")

	me := expect(t, call(t, "GET", "/api/v1/me", nina, nil), 200, "chairman me")
	if me.body["chairman"] != true {
		t.Fatalf("chairman me = %v", me.body)
	}
	if r := expect(t, call(t, "GET", "/api/v1/me", anna, nil), 200, "resident me"); r.body["chairman"] != nil {
		t.Fatalf("resident me has chairman flag: %v", r.body)
	}

	p := expect(t, call(t, "POST", "/api/v1/proposals", anna, map[string]string{"text": "Повесить доску объявлений в первом подъезде"}), 201, "propose")
	pid := p.body["id"].(string)
	expect(t, call(t, "POST", "/api/v1/proposals", anna, map[string]string{"text": "мало"}), 422, "short proposal")
	expect(t, call(t, "POST", "/api/v1/proposals", oper, map[string]string{"text": "Повесить доску объявлений"}), 403, "operator proposes")

	folder := expect(t, call(t, "GET", "/api/v1/council/proposals", nina, nil), 200, "folder")
	if len(folder.list) < 4 {
		t.Fatalf("folder = %v", folder.list)
	}
	first := folder.list[0].(map[string]any)
	if first["status"] != "new" || strings.Contains(first["text"].(string), "Анна") || first["author_id"] != nil {
		t.Fatalf("folder item = %v", first)
	}
	expect(t, call(t, "GET", "/api/v1/council/proposals", anna, nil), 403, "resident opens folder")
	expect(t, call(t, "GET", "/api/v1/council/proposals", district, nil), 403, "district opens folder")

	expect(t, call(t, "POST", "/api/v1/council/proposals/"+pid+"/reply", anna, map[string]string{"status": "accepted"}), 404, "resident replies")
	expect(t, call(t, "POST", "/api/v1/council/proposals/"+pid+"/reply", nina, map[string]string{"status": "declined"}), 422, "decline without answer")
	r := expect(t, call(t, "POST", "/api/v1/council/proposals/"+pid+"/reply", nina, map[string]string{"status": "accepted", "answer": "Закажу доску"}), 200, "reply")
	if r.body["status"] != "accepted" || r.body["answered_at"] == nil {
		t.Fatalf("reply = %v", r.body)
	}
	expect(t, call(t, "POST", "/api/v1/council/proposals/"+pid+"/reply", nina, map[string]string{"status": "accepted"}), 409, "second reply")
	mine := expect(t, call(t, "GET", "/api/v1/me/proposals", anna, nil), 200, "my proposals")
	if len(mine.list) == 0 || mine.list[0].(map[string]any)["answer"] != "Закажу доску" {
		t.Fatalf("my proposals = %v", mine.list)
	}

	poll := expect(t, call(t, "POST", "/api/v1/council/polls", nina, map[string]any{
		"proposal_id": pid, "question": "Где повесить доску?", "options": []string{"У лифта", "У почтовых ящиков"},
	}), 201, "create poll")
	pollID := poll.body["id"].(string)
	if poll.body["open"] != true || poll.body["my_vote"] != nil || len(poll.body["options"].([]any)) != 2 {
		t.Fatalf("poll = %v", poll.body)
	}
	expect(t, call(t, "POST", "/api/v1/council/polls", anna, map[string]any{"question": "Где доска?", "options": []string{"А", "Б"}}), 403, "resident creates poll")
	expect(t, call(t, "POST", "/api/v1/council/polls", nina, map[string]any{"question": "Где доска?", "options": []string{"А"}}), 422, "one option")

	v := expect(t, call(t, "POST", "/api/v1/polls/"+pollID+"/vote", sergey, map[string]int{"option": 1}), 200, "vote")
	if v.body["total"] != 1.0 || v.body["my_vote"] != 1.0 {
		t.Fatalf("vote = %v", v.body)
	}
	expect(t, call(t, "POST", "/api/v1/polls/"+pollID+"/vote", sergey, map[string]int{"option": 0}), 409, "second vote")
	expect(t, call(t, "POST", "/api/v1/polls/"+pollID+"/vote", anna, map[string]int{"option": 5}), 422, "missing option")
	expect(t, call(t, "POST", "/api/v1/polls/"+pollID+"/vote", anna, map[string]any{}), 422, "no option")
	expect(t, call(t, "POST", "/api/v1/polls/"+pollID+"/vote", oper, map[string]int{"option": 0}), 403, "operator votes")
	expect(t, call(t, "POST", "/api/v1/polls/abc/vote", anna, map[string]int{"option": 0}), 404, "poll id not a uuid")

	polls := expect(t, call(t, "GET", "/api/v1/polls", anna, nil), 200, "polls")
	var sample map[string]any
	for _, item := range polls.list {
		if m := item.(map[string]any); m["question"] == "Ставим шлагбаум на въезде во двор?" {
			sample = m
		}
	}
	if sample == nil || sample["total"] != 7.0 || sample["open"] != true {
		t.Fatalf("sample poll = %v", sample)
	}
	expect(t, call(t, "GET", "/api/v1/polls", district, nil), 403, "district polls")
	expect(t, call(t, "GET", "/api/v1/polls", "", nil), 401, "polls without token")
}
