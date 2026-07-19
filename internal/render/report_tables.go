package render

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	htmlpkg "html"
	"strconv"
	"strings"

	"github.com/dotcommander/html/internal/report"
)

func dataTable(src []byte, analysis report.Analysis) string {
	headers, rows := tableRows(src, analysis)
	if len(headers) == 0 {
		return rawPre(src)
	}
	labels := headerLabels(headers)
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="report-table-wrap"><input class="report-filter" type="search" placeholder="Filter rows" aria-label="Filter rows"><div class="report-mobile-sort"><select aria-label="Sort rows"><option value="">Sort rows</option>`)
	for i, label := range labels {
		fmt.Fprintf(&b, `<option value="%d:ascending">%s ↑</option><option value="%d:descending">%s ↓</option>`, i, escapeTableText(label), i, escapeTableText(label))
	}
	fmt.Fprintf(&b, `</select></div><p class="report-filter-status" aria-live="polite">%s</p><table class="report-table" data-report-table><thead><tr>`, rowStatusText(len(rows)))
	for _, label := range labels {
		fmt.Fprintf(&b, `<th scope="col"><button type="button" data-sort-label="%s" aria-label="Sort by %s ascending">`, escapeTableText(label), escapeTableText(label))
		b.WriteString(escapeTableText(label))
		b.WriteString(`</button></th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, row := range rows {
		b.WriteString(`<tr>`)
		for i := range headers {
			label := ""
			if i < len(labels) {
				label = labels[i]
			}
			fmt.Fprintf(&b, `<td data-label="%s">`, escapeTableText(label))
			if i < len(row) {
				b.WriteString(escapeTableText(row[i]))
			}
			b.WriteString(`</td>`)
		}
		b.WriteString(`</tr>`)
	}
	fmt.Fprintf(&b, `<tr class="report-empty-row" data-report-empty-row%s><td colspan="%d">%s</td></tr>`, hiddenAttr(len(rows) > 0), len(headers), emptyTableText(len(rows) == 0))
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

func rowStatusText(n int) string {
	if n == 1 {
		return "1 row"
	}
	return fmt.Sprintf("%d rows", n)
}

func hiddenAttr(hidden bool) string {
	if hidden {
		return ` hidden`
	}
	return ""
}

func emptyTableText(emptyInput bool) string {
	if emptyInput {
		return "No rows"
	}
	return "No rows match"
}

func recordCards(src []byte, analysis report.Analysis) string {
	headers, rows := tableRows(src, analysis)
	if len(headers) == 0 {
		return rawPre(src)
	}
	if len(rows) == 0 {
		return `<p class="record-empty" aria-live="polite">No records</p>`
	}
	labels := headerLabels(headers)
	cards := make([]recordCard, 0, len(rows))
	stats := recordCardStats{Cards: len(rows)}
	for i, row := range rows {
		card := recordCard{Title: recordCardTitle(i+1, labels, row)}
		for j, label := range labels {
			if j >= len(row) || strings.TrimSpace(cleanTableText(row[j])) == "" {
				continue
			}
			card.Fields = append(card.Fields, recordCardField{Label: label, Value: row[j]})
			stats.VisibleFields++
		}
		cards = append(cards, card)
	}
	var b strings.Builder
	b.WriteString(recordCardsOverview(stats))
	b.WriteString(`<div class="record-cards">`)
	for _, card := range cards {
		fmt.Fprintf(&b, `<article class="record-card"><h3>%s</h3><dl>`, card.Title)
		for _, field := range card.Fields {
			b.WriteString(`<div><dt>`)
			b.WriteString(escapeTableText(field.Label))
			b.WriteString(`</dt><dd>`)
			b.WriteString(escapeTableText(field.Value))
			b.WriteString(`</dd></div>`)
		}
		b.WriteString(`</dl></article>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

type recordCard struct {
	Title  string
	Fields []recordCardField
}

type recordCardField struct {
	Label string
	Value string
}

type recordCardStats struct {
	Cards         int
	VisibleFields int
}

func recordCardsOverview(stats recordCardStats) string {
	var b strings.Builder
	b.WriteString(`<dl class="record-cards-overview" aria-label="Record cards overview">`)
	for _, item := range [][2]string{
		{"Cards", strconv.Itoa(stats.Cards)},
		{"Visible fields", strconv.Itoa(stats.VisibleFields)},
	} {
		b.WriteString(`<div><dt>`)
		b.WriteString(htmlpkg.EscapeString(item[0]))
		b.WriteString(`</dt><dd>`)
		b.WriteString(htmlpkg.EscapeString(item[1]))
		b.WriteString(`</dd></div>`)
	}
	b.WriteString(`</dl>`)
	return b.String()
}

func recordCardTitle(n int, labels, row []string) string {
	fallback := fmt.Sprintf("Record %d", n)
	for _, preferred := range []string{"name", "title", "id", "key"} {
		for i, label := range labels {
			if !strings.EqualFold(strings.TrimSpace(label), preferred) || i >= len(row) {
				continue
			}
			value := strings.TrimSpace(cleanTableText(row[i]))
			if value == "" {
				continue
			}
			return escapeTableText(fallback + ": " + value)
		}
	}
	return escapeTableText(fallback)
}

// recordLabel derives a human-readable per-record label for comment exports,
// reusing recordCardTitle's preferred-field order (name, title, id, key) and
// falling back to the 1-based record number.
func recordLabel(n int, labels, row []string) string {
	for _, preferred := range []string{"name", "title", "id", "key"} {
		for i, label := range labels {
			if !strings.EqualFold(strings.TrimSpace(label), preferred) || i >= len(row) {
				continue
			}
			value := strings.TrimSpace(cleanTableText(row[i]))
			if value == "" {
				continue
			}
			return value
		}
	}
	return fmt.Sprintf("record-%d", n)
}

func reviewDocumentDigest(src []byte) string {
	sum := sha256.Sum256(src)
	return hex.EncodeToString(sum[:])
}

// canonicalRowDigest hashes the complete presented row shape rather than its
// display label. JSON array encoding keeps field boundaries unambiguous.
func canonicalRowDigest(labels, row []string) string {
	canonical, _ := json.Marshal([][]string{labels, row})
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

// reviewCards mirrors recordCards but adds a per-record comment <textarea>
// (keyed by document, canonical row, and duplicate occurrence for localStorage
// persistence) and a single top-level "Copy all comments" button. The
// interactive behavior lives in report.js.
func reviewCards(src []byte, analysis report.Analysis) string {
	headers, rows := tableRows(src, analysis)
	if len(headers) == 0 {
		return rawPre(src)
	}
	if len(rows) == 0 {
		return `<p class="record-empty" aria-live="polite">No records</p>`
	}
	labels := headerLabels(headers)
	type reviewItem struct {
		card       recordCard
		rowDigest  string
		occurrence int
		label      string
	}
	items := make([]reviewItem, 0, len(rows))
	occurrences := make(map[string]int)
	stats := recordCardStats{Cards: len(rows)}
	for i, row := range rows {
		card := recordCard{Title: recordCardTitle(i+1, labels, row)}
		for j, label := range labels {
			if j >= len(row) || strings.TrimSpace(cleanTableText(row[j])) == "" {
				continue
			}
			card.Fields = append(card.Fields, recordCardField{Label: label, Value: row[j]})
			stats.VisibleFields++
		}
		rowDigest := canonicalRowDigest(labels, row)
		occurrences[rowDigest]++
		items = append(items, reviewItem{
			card:       card,
			rowDigest:  rowDigest,
			occurrence: occurrences[rowDigest],
			label:      recordLabel(i+1, labels, row),
		})
	}
	documentDigest := reviewDocumentDigest(src)
	var b strings.Builder
	b.WriteString(recordCardsOverview(stats))
	b.WriteString(`<button type="button" class="review-copy">Copy all comments</button>`)
	b.WriteString(`<div class="review-cards">`)
	for _, item := range items {
		fmt.Fprintf(&b, `<article class="review-card"><h3>%s</h3><dl>`, item.card.Title)
		for _, field := range item.card.Fields {
			b.WriteString(`<div><dt>`)
			b.WriteString(escapeTableText(field.Label))
			b.WriteString(`</dt><dd>`)
			b.WriteString(escapeTableText(field.Value))
			b.WriteString(`</dd></div>`)
		}
		fmt.Fprintf(&b, `</dl><textarea class="review-comment" data-review-document="%s" data-review-row="%s" data-review-occurrence="%d" data-review-label="%s" aria-label="Comment"></textarea></article>`, documentDigest, item.rowDigest, item.occurrence, htmlpkg.EscapeString(item.label))
	}
	b.WriteString(`</div>`)
	return b.String()
}
