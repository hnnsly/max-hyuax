package main

import (
	"strings"
	"testing"
)

func TestParseHousesCSV(t *testing.T) {
	// Excel в русской локали сохраняет CSV через точку с запятой и с BOM: такой файл тоже читается.
	in := string(rune(0xFEFF)) + "id;address;district;org_id;year;floors;entrances;lat;lon;source\n" +
		";Ореховый бульвар, 15;Зябликово;org-orekh;1979;9;4;;;model\n" +
		"h-x;Ясеневая улица, 1;Зябликово;org-yasen;;12;2;55,62;37.75;\n" +
		"h-y;Улица, 2;Зябликово;org-yasen;год;9;2;;;\n"
	rows, bad, err := parseHousesCSV(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || len(bad) != 2 {
		t.Fatalf("rows = %+v, bad = %+v", rows, bad)
	}
	h := rows[0].House
	if rows[0].Line != 2 || h.Address != "Ореховый бульвар, 15" || h.Floors != 9 || h.EntrancesCount != 4 || h.Source != "model" || h.Lat != 0 {
		t.Fatalf("row 2 = %+v", rows[0])
	}
	// Десятичная запятая в координатах — ошибка строки 3, а не молчаливый ноль.
	if bad[0].Line != 3 || !strings.Contains(bad[0].Reason, "lat") || bad[1].Line != 4 || !strings.Contains(bad[1].Reason, "year") {
		t.Fatalf("bad = %+v", bad)
	}
}

func TestParseHousesCSVHeader(t *testing.T) {
	if _, _, err := parseHousesCSV(strings.NewReader("address,district\nУлица, 1,Зябликово\n")); err == nil || !strings.Contains(err.Error(), "org_id") {
		t.Fatalf("missing column err = %v", err)
	}
	if _, _, err := parseHousesCSV(strings.NewReader("")); err == nil {
		t.Fatal("empty file accepted")
	}
	rows, _, err := parseHousesCSV(strings.NewReader("address,district,org_id\n\"Улица, 1\",Зябликово,org-1\n"))
	if err != nil || len(rows) != 1 || rows[0].House.Address != "Улица, 1" {
		t.Fatalf("comma CSV rows = %+v, err = %v", rows, err)
	}
}
