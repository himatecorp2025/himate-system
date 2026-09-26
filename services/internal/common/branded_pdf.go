package common

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
)

const (
	brandedPDFWidth  = 792.0
	brandedPDFHeight = 612.0
)

func pdfLatin1(input string) string {
	var out strings.Builder
	for _, r := range input {
		switch r {
		case 'ő', 'Ő':
			r = 'o'
		case 'ű', 'Ű':
			r = 'u'
		case '\n', '\r', '\t':
			r = ' '
		}
		switch {
		case r == '\\' || r == '(' || r == ')':
			out.WriteByte('\\')
			out.WriteRune(r)
		case r >= 32 && r <= 255:
			out.WriteByte(byte(r))
		default:
			out.WriteByte('?')
		}
	}
	return out.String()
}

func pdfText(s string) string {
	return "(" + pdfLatin1(strings.TrimSpace(s)) + ")"
}

func truncatePDFCell(value string, width float64, fontSize float64) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "-"
	}
	maxChars := int(width / (fontSize * 0.52))
	if maxChars < 4 {
		maxChars = 4
	}
	runes := []rune(value)
	if len(runes) <= maxChars {
		return value
	}
	if maxChars <= 3 {
		return string(runes[:maxChars])
	}
	return string(runes[:maxChars-3]) + "..."
}

func tableColumnWidths(columns []string, rows [][]string, available float64) []float64 {
	if len(columns) == 0 {
		return nil
	}
	weights := make([]float64, len(columns))
	total := 0.0
	for i, column := range columns {
		weight := float64(len([]rune(column)))
		if weight < 7 {
			weight = 7
		}
		if weight > 22 {
			weight = 22
		}
		for rowIndex, row := range rows {
			if rowIndex >= 80 || i >= len(row) {
				break
			}
			n := float64(len([]rune(strings.Join(strings.Fields(row[i]), " "))))
			if n > weight {
				weight = n
			}
		}
		if weight > 28 {
			weight = 28
		}
		weights[i] = weight
		total += weight
	}
	widths := make([]float64, len(columns))
	for i, weight := range weights {
		widths[i] = available * weight / total
	}
	return widths
}

func brandedPDFPage(title, subtitle string, columns []string, widths []float64, rows [][]string, page, pages int) string {
	const (
		left       = 28.0
		right      = 28.0
		tableTop   = 503.0
		headerH    = 24.0
		rowH       = 18.0
		cellPad    = 4.0
		bodyFont   = 7.0
		headerFont = 7.4
	)
	var b strings.Builder

	// Deep IT-blue canvas.
	b.WriteString("0.010 0.035 0.075 rg 0 0 792 612 re f\n")
	// Header.
	b.WriteString("0.020 0.090 0.180 rg 0 535 792 77 re f\n")
	b.WriteString("0.830 0.680 0.260 rg 0 532 792 3 re f\n")
	b.WriteString("BT /F2 20 Tf 1 1 1 rg 30 575 Td ")
	b.WriteString(pdfText(title))
	b.WriteString(" Tj ET\n")
	b.WriteString("BT /F1 9 Tf 0.72 0.82 0.94 rg 30 556 Td ")
	b.WriteString(pdfText(subtitle))
	b.WriteString(" Tj ET\n")

	// Table shell + header.
	b.WriteString(fmt.Sprintf("0.030 0.115 0.220 rg %.1f %.1f %.1f %.1f re f\n", left, tableTop-headerH, brandedPDFWidth-left-right, headerH))
	x := left
	for i, column := range columns {
		width := widths[i]
		b.WriteString(fmt.Sprintf("BT /F2 %.1f Tf 0.88 0.94 1 rg %.1f %.1f Td %s Tj ET\n",
			headerFont, x+cellPad, tableTop-16, pdfText(truncatePDFCell(column, width-cellPad*2, headerFont))))
		x += width
		if i < len(columns)-1 {
			b.WriteString(fmt.Sprintf("0.10 0.28 0.46 RG 0.45 w %.1f %.1f m %.1f %.1f l S\n", x, tableTop-headerH, x, tableTop))
		}
	}

	y := tableTop - headerH
	for rowIndex, row := range rows {
		y -= rowH
		if rowIndex%2 == 0 {
			b.WriteString(fmt.Sprintf("0.018 0.075 0.145 rg %.1f %.1f %.1f %.1f re f\n", left, y, brandedPDFWidth-left-right, rowH))
		} else {
			b.WriteString(fmt.Sprintf("0.025 0.105 0.195 rg %.1f %.1f %.1f %.1f re f\n", left, y, brandedPDFWidth-left-right, rowH))
		}
		x = left
		for colIndex := range columns {
			width := widths[colIndex]
			value := ""
			if colIndex < len(row) {
				value = row[colIndex]
			}
			b.WriteString(fmt.Sprintf("BT /F1 %.1f Tf 0.88 0.93 1 rg %.1f %.1f Td %s Tj ET\n",
				bodyFont, x+cellPad, y+6, pdfText(truncatePDFCell(value, width-cellPad*2, bodyFont))))
			x += width
		}
	}

	// Footer.
	b.WriteString("0.10 0.25 0.40 RG 0.5 w 28 29 m 764 29 l S\n")
	b.WriteString("BT /F1 7 Tf 0.55 0.68 0.82 rg 28 16 Td (HiMate Central - confidential operational export) Tj ET\n")
	b.WriteString(fmt.Sprintf("BT /F1 7 Tf 0.55 0.68 0.82 rg 700 16 Td (Page %d / %d) Tj ET\n", page, pages))
	return b.String()
}

// WriteBrandedTablePDF renders a dark, HiMate-branded, multi-page PDF table.
// It deliberately uses built-in PDF fonts so exports have zero runtime dependency
// on external font files or document services.
func WriteBrandedTablePDF(
	w http.ResponseWriter,
	filename string,
	title string,
	subtitle string,
	columns []string,
	rows [][]string,
) {
	const rowsPerPage = 24
	if len(columns) == 0 {
		columns = []string{"Result"}
	}
	if len(rows) == 0 {
		rows = [][]string{{"No records matched the selected filters."}}
		if len(columns) > 1 {
			row := make([]string, len(columns))
			row[0] = "No records matched the selected filters."
			rows = [][]string{row}
		}
	}

	pageCount := (len(rows) + rowsPerPage - 1) / rowsPerPage
	widths := tableColumnWidths(columns, rows, brandedPDFWidth-56)

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"", // pages object is filled after page object numbers are known.
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>",
	}
	pageObjectNumbers := make([]int, 0, pageCount)

	for pageIndex := 0; pageIndex < pageCount; pageIndex++ {
		start := pageIndex * rowsPerPage
		end := start + rowsPerPage
		if end > len(rows) {
			end = len(rows)
		}
		stream := brandedPDFPage(title, subtitle, columns, widths, rows[start:end], pageIndex+1, pageCount)
		pageObj := len(objects) + 1
		contentObj := pageObj + 1
		pageObjectNumbers = append(pageObjectNumbers, pageObj)
		objects = append(objects,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>", brandedPDFWidth, brandedPDFHeight, contentObj),
			fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
		)
	}

	var kids strings.Builder
	for _, objectNumber := range pageObjectNumbers {
		fmt.Fprintf(&kids, "%d 0 R ", objectNumber)
	}
	objects[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids.String(), pageCount)

	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%HiMate Central\n")
	offsets := make([]int, len(objects)+1)
	for i, object := range objects {
		offsets[i+1] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for i := 1; i <= len(objects); i++ {
		fmt.Fprintf(&out, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&out, "trailer << /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(objects)+1, xref)

	if !strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		filename += ".pdf"
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out.Bytes())
}
