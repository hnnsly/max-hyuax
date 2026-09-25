package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"dommax/internal/app/houses"
)

// maxCSVBytes — предел файла реестра: район Москвы — это сотни домов, а не сотни мегабайт.
const maxCSVBytes = 10 << 20

// importHouses — команда import-houses: реестр домов из CSV-файла или из stdin («-»), ADR-016.
// Отчёт печатается в out; строки с ошибками не мешают остальным.
func importHouses(ctx context.Context, cfg config, log *slog.Logger, path string, out io.Writer) error {
	in := io.Reader(os.Stdin)
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		in = f
	}
	rows, bad, err := parseHousesCSV(in)
	if err != nil {
		return err
	}
	c, err := Open(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer c.Close()
	rep, err := c.Importer().Import(ctx, rows)
	rep.Errors = append(bad, rep.Errors...)
	fmt.Fprintf(out, "Импорт домов: создано %d, обновлено %d, координаты найдены у %d, без координат %d, ошибок %d\n",
		rep.Created, rep.Updated, rep.Geocoded, rep.WithoutCoords, len(rep.Errors))
	for _, e := range rep.Errors {
		fmt.Fprintf(out, "строка %d: %s\n", e.Line, e.Reason)
	}
	if rep.Geocoded > 0 {
		fmt.Fprintln(out, "Координаты: © участники OpenStreetMap, ODbL")
	}
	return err
}

// Обязательные и необязательные колонки реестра; порядок колонок в файле любой.
var (
	requiredColumns = []string{"address", "district", "org_id"}
	optionalColumns = []string{"id", "year", "floors", "entrances", "lat", "lon", "source"}
)

// parseHousesCSV читает реестр. Разделитель — запятая или точка с запятой (так сохраняет Excel
// в русской локали), BOM в начале файла пропускается. Ошибка значения — ошибка строки, а не файла.
func parseHousesCSV(r io.Reader) ([]houses.ImportRow, []houses.RowError, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxCSVBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if len(data) > maxCSVBytes {
		return nil, nil, fmt.Errorf("csv is larger than %d MB", maxCSVBytes>>20)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // BOM UTF-8
	header, _, _ := bytes.Cut(data, []byte("\n"))
	cr := csv.NewReader(bufio.NewReader(bytes.NewReader(data)))
	if bytes.Count(header, []byte(";")) > bytes.Count(header, []byte(",")) {
		cr.Comma = ';'
	}
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	names, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil, errors.New("csv is empty: want a header line")
	}
	if err != nil {
		return nil, nil, err
	}
	col := map[string]int{}
	for i, n := range names {
		col[strings.ToLower(strings.TrimSpace(n))] = i
	}
	var missing []string
	for _, n := range requiredColumns {
		if _, ok := col[n]; !ok {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return nil, nil, fmt.Errorf("csv header: missing columns %s (optional: %s)",
			strings.Join(missing, ", "), strings.Join(optionalColumns, ", "))
	}

	var rows []houses.ImportRow
	var bad []houses.RowError
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		line, _ := cr.FieldPos(0)
		if err != nil {
			bad = append(bad, houses.RowError{Line: line, Reason: "строка CSV не читается: " + err.Error()})
			continue
		}
		row, reason := parseHouseRecord(rec, col)
		if reason != "" {
			bad = append(bad, houses.RowError{Line: line, Reason: reason})
			continue
		}
		row.Line = line
		rows = append(rows, row)
	}
	return rows, bad, nil
}

func parseHouseRecord(rec []string, col map[string]int) (houses.ImportRow, string) {
	get := func(name string) string {
		if i, ok := col[name]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
	var reasons []string
	num := func(name string, parse func(string) error) {
		if v := get(name); v != "" {
			if err := parse(v); err != nil {
				reasons = append(reasons, fmt.Sprintf("колонка %s: %q не число", name, v))
			}
		}
	}
	var row houses.ImportRow
	h := &row.House
	h.ID, h.Address, h.District, h.OrganizationID, h.Source = get("id"), get("address"), get("district"), get("org_id"), get("source")
	atoi := func(dst *int) func(string) error {
		return func(s string) (err error) { *dst, err = strconv.Atoi(s); return err }
	}
	atof := func(dst *float64) func(string) error {
		return func(s string) (err error) { *dst, err = strconv.ParseFloat(s, 64); return err }
	}
	num("year", atoi(&h.YearBuilt))
	num("floors", atoi(&h.Floors))
	num("entrances", atoi(&h.EntrancesCount))
	num("lat", atof(&h.Lat))
	num("lon", atof(&h.Lon))
	return row, strings.Join(reasons, "; ")
}
