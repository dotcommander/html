package report

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnalyze_DeterministicKinds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		input      string
		sourceName string
		wantKind   Kind
		wantType   ComponentType
	}{
		{
			name:     "json records table",
			input:    `[{"name":"a","score":1},{"name":"b","score":2}]`,
			wantKind: KindJSONRecords,
			wantType: ComponentDataTable,
		},
		{
			name:     "bom json records table",
			input:    "\ufeff" + `[{"name":"a","score":1}]`,
			wantKind: KindJSONRecords,
			wantType: ComponentDataTable,
		},
		{
			name:       "whitespace before bom is not json records",
			input:      " \n\ufeff" + `[{"name":"a","score":1}]`,
			sourceName: "bad.json",
			wantKind:   KindSourceCode,
			wantType:   ComponentCodeBlock,
		},
		{
			name:     "json heterogeneous records cards",
			input:    `[{"name":"a"},{"score":2}]`,
			wantKind: KindJSONRecords,
			wantType: ComponentRecordCards,
		},
		{
			name:       "json lines records table",
			input:      "{\"name\":\"a\",\"score\":1}\n{\"name\":\"b\",\"score\":2}\n",
			sourceName: "events.jsonl",
			wantKind:   KindJSONRecords,
			wantType:   ComponentDataTable,
		},
		{
			name:       "ndjson records table",
			input:      "{\"name\":\"a\",\"score\":1}\n{\"name\":\"b\",\"score\":2}\n",
			sourceName: "events.ndjson",
			wantKind:   KindJSONRecords,
			wantType:   ComponentDataTable,
		},
		{
			name:       "single json line record table",
			input:      "{\"name\":\"a\",\"score\":1}\n",
			sourceName: "events.jsonl",
			wantKind:   KindJSONRecords,
			wantType:   ComponentDataTable,
		},
		{
			name:       "single ndjson record table",
			input:      "{\"name\":\"a\",\"score\":1}\n",
			sourceName: "events.ndjson",
			wantKind:   KindJSONRecords,
			wantType:   ComponentDataTable,
		},
		{
			name:       "single jsonlines record table",
			input:      "{\"name\":\"a\",\"score\":1}\n",
			sourceName: "events.jsonlines",
			wantKind:   KindJSONRecords,
			wantType:   ComponentDataTable,
		},
		{
			name:       "json lines heterogeneous records cards",
			input:      "{\"name\":\"a\"}\n{\"score\":2}\n",
			sourceName: "events.jsonl",
			wantKind:   KindJSONRecords,
			wantType:   ComponentRecordCards,
		},
		{
			name:       "mixed json lines are not records",
			input:      "{\"name\":\"a\"}\n2\n",
			sourceName: "bad.jsonl",
			wantKind:   KindSourceCode,
			wantType:   ComponentCodeBlock,
		},
		{
			name:     "json scalar array raw json",
			input:    `[1,2,3]`,
			wantKind: KindJSONObject,
			wantType: ComponentRawJSON,
		},
		{
			name:       "json scalar file raw json",
			input:      `true`,
			sourceName: "value.json",
			wantKind:   KindJSONObject,
			wantType:   ComponentRawJSON,
		},
		{
			name:     "bare scalar stdin remains plain",
			input:    `true`,
			wantKind: KindPlain,
			wantType: ComponentPreformatted,
		},
		{
			name:     "empty json array raw json",
			input:    `[]`,
			wantKind: KindJSONObject,
			wantType: ComponentRawJSON,
		},
		{
			name:     "json empty object array raw json",
			input:    `[{},{}]`,
			wantKind: KindJSONObject,
			wantType: ComponentRawJSON,
		},
		{
			name:       "json with trailing text is not records",
			input:      `[{"a":1}] trailing`,
			sourceName: "bad.json",
			wantKind:   KindSourceCode,
			wantType:   ComponentCodeBlock,
		},
		{
			name:     "csv records",
			input:    "name,score\na,1\nb,2\n",
			wantKind: KindCSVRecords,
			wantType: ComponentDataTable,
		},
		{
			name:       "csv header-only data file",
			input:      "name,score\n",
			sourceName: "empty.csv",
			wantKind:   KindCSVRecords,
			wantType:   ComponentDataTable,
		},
		{
			name:     "single comma line stdin remains plain",
			input:    "hello, world\n",
			wantKind: KindPlain,
			wantType: ComponentPreformatted,
		},
		{
			name:     "timestamped csv records beat log markers",
			input:    "time,level,msg\n2026-01-01 12:00:00,ERROR,oops\n2026-01-01 12:01:00,INFO,ok\n",
			wantKind: KindCSVRecords,
			wantType: ComponentDataTable,
		},
		{
			name:     "bom csv records",
			input:    "\ufeff" + "name,score\na,1\n",
			wantKind: KindCSVRecords,
			wantType: ComponentDataTable,
		},
		{
			name: "mysql boxed table records",
			input: "+----+-------+\n" +
				"| id | name  |\n" +
				"+----+-------+\n" +
				"| 1  | alpha |\n" +
				"| 2  | beta  |\n" +
				"+----+-------+\n",
			wantKind: KindTableRecords,
			wantType: ComponentDataTable,
		},
		{
			name: "psql aligned table records",
			input: " id | name\n" +
				"----+-------\n" +
				"  1 | alpha\n" +
				"  2 | beta\n" +
				"(2 rows)\n",
			wantKind: KindTableRecords,
			wantType: ComponentDataTable,
		},
		{
			name:       "whitespace before bom is not csv records",
			input:      " \n\ufeff" + "name,score\na,1\n",
			sourceName: "bad.csv",
			wantKind:   KindSourceCode,
			wantType:   ComponentCodeBlock,
		},
		{
			name:     "diff",
			input:    "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-a\n+b\n",
			wantKind: KindDiff,
			wantType: ComponentDiffView,
		},
		{
			name:     "plain unified diff starts with file headers",
			input:    "--- old.txt\n+++ new.txt\n@@ -1 +1 @@\n-old\n+new\n",
			wantKind: KindDiff,
			wantType: ComponentDiffView,
		},
		{
			name:     "combined git diff",
			input:    "diff --cc main.go\nindex 1111111,2222222..3333333\n--- a/main.go\n+++ b/main.go\n@@@ -1,1 -1,1 +1,1 @@@\n- left\n -right\n++merged\n",
			wantKind: KindDiff,
			wantType: ComponentDiffView,
		},
		{
			name:     "git binary patch without hunk",
			input:    "diff --git a/logo.png b/logo.png\nnew file mode 100644\nindex 0000000..1111111\nGIT binary patch\nliteral 0\nHcmV?d00001\n",
			wantKind: KindDiff,
			wantType: ComponentDiffView,
		},
		{
			name:     "git mode-only patch without hunk",
			input:    "diff --git a/script.sh b/script.sh\nold mode 100644\nnew mode 100755\n",
			wantKind: KindDiff,
			wantType: ComponentDiffView,
		},
		{
			name:     "git copy-only patch without hunk",
			input:    "diff --git a/source.txt b/copy.txt\ncopy from source.txt\ncopy to copy.txt\n",
			wantKind: KindDiff,
			wantType: ComponentDiffView,
		},
		{
			name:       "patch filename renders diff",
			input:      "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-a\n+b\n",
			sourceName: "change.patch",
			wantKind:   KindDiff,
			wantType:   ComponentDiffView,
		},
		{
			name:       "go source",
			input:      "package main\n\nfunc main() {}\n",
			sourceName: "main.go",
			wantKind:   KindSourceCode,
			wantType:   ComponentCodeBlock,
		},
		{
			name:       "go source with markdown fence string",
			input:      "package main\n\nvar doc = `\n```go\nfmt.Println()\n```\n`\n",
			sourceName: "main.go",
			wantKind:   KindSourceCode,
			wantType:   ComponentCodeBlock,
		},
		{
			name:       "go source filename beats csv body",
			input:      "name,score\na,1\nb,2\n",
			sourceName: "main.go",
			wantKind:   KindSourceCode,
			wantType:   ComponentCodeBlock,
		},
		{
			name:       "go source filename beats json body",
			input:      `[{"name":"a","score":1}]`,
			sourceName: "main.go",
			wantKind:   KindSourceCode,
			wantType:   ComponentCodeBlock,
		},
		{
			name:     "tree listing",
			input:    ".\n├── cmd\n│   └── html\n└── internal\n",
			wantKind: KindTreeListing,
			wantType: ComponentFileTree,
		},
		{
			name:       "path listing with unknown extension",
			input:      "./cmd/html\n./internal/render\n./internal/report\n",
			sourceName: "list.weird",
			wantKind:   KindTreeListing,
			wantType:   ComponentFileTree,
		},
		{
			name:       "windows path listing with unknown extension",
			input:      "cmd\\html\ninternal\\render\ninternal\\report\n",
			sourceName: "list.weird",
			wantKind:   KindTreeListing,
			wantType:   ComponentFileTree,
		},
		{
			name:       "windows drive path listing with unknown extension",
			input:      "C:\\Users\\Alice\nC:\\Users\\Alice\\Documents\nD:\\Archive\\Logs\n",
			sourceName: "list.weird",
			wantKind:   KindTreeListing,
			wantType:   ComponentFileTree,
		},
		{
			name:       "numeric path listing with unknown extension",
			input:      "./2024/01\n./2024/02\n./2025/01\n",
			sourceName: "list.weird",
			wantKind:   KindTreeListing,
			wantType:   ComponentFileTree,
		},
		{
			name:       "absolute path listing with unknown extension",
			input:      "/usr/bin\n/usr/local/bin\n/var/log/html\n",
			sourceName: "list.weird",
			wantKind:   KindTreeListing,
			wantType:   ComponentFileTree,
		},
		{
			name:       "parent relative path listing with unknown extension",
			input:      "../src/main.go\n../src/internal/render.go\n../docs/README.md\n",
			sourceName: "list.weird",
			wantKind:   KindTreeListing,
			wantType:   ComponentFileTree,
		},
		{
			name:       "home relative path listing with unknown extension",
			input:      "~/src/main.go\n~/src/internal/render.go\n~/docs/README.md\n",
			sourceName: "list.weird",
			wantKind:   KindTreeListing,
			wantType:   ComponentFileTree,
		},
		{
			name:       "path listing with spaces",
			input:      "./My Project/README.md\n./My Project/docs/Release Notes.md\n./Other Project/TODO.txt\n",
			sourceName: "list.weird",
			wantKind:   KindTreeListing,
			wantType:   ComponentFileTree,
		},
		{
			name:     "url list is not a file tree",
			input:    "https://example.com/a\nhttps://example.com/b\nhttps://example.com/c\n",
			wantKind: KindPlain,
			wantType: ComponentPreformatted,
		},
		{
			name:     "fractions are not a file tree",
			input:    "1/2 complete\n2/3 done\n3/4 ready\n",
			wantKind: KindPlain,
			wantType: ComponentPreformatted,
		},
		{
			name:     "bare fractions are not a file tree",
			input:    "1/2\n2/3\n3/4\n",
			wantKind: KindPlain,
			wantType: ComponentPreformatted,
		},
		{
			name:     "http request paths are not a file tree",
			input:    "GET /api/users\nPOST /api/tasks\nDELETE /api/tasks/1\n",
			wantKind: KindPlain,
			wantType: ComponentPreformatted,
		},
		{
			name:     "dash divider is not markdown",
			input:    "Results\n--------------------\nrow 1\nrow 2\n",
			wantKind: KindPlain,
			wantType: ComponentPreformatted,
		},
		{
			name:     "ascii tree listing",
			input:    ".\n|-- cmd\n|   `-- html\n`-- internal\n",
			wantKind: KindTreeListing,
			wantType: ComponentFileTree,
		},
		{
			name:     "log",
			input:    "2026-06-16 12:00:00 INFO start\n2026-06-16 12:00:01 ERROR stop\n",
			wantKind: KindLog,
			wantType: ComponentPreformatted,
		},
		{
			name:     "speaker transcript",
			input:    "Host: Welcome back to the show.\nGuest: Thanks for having me.\nHost: Let's start with the launch.\nGuest: The first release is ready.\n",
			wantKind: KindTranscript,
			wantType: ComponentPreformatted,
		},
		{
			name:     "speaker transcript with generic labels",
			input:    "Speaker 1: We should verify the HTML output.\nSpeaker 2: I have the mobile screenshots.\nSpeaker 1: The controls no longer overlap.\n",
			wantKind: KindTranscript,
			wantType: ComponentPreformatted,
		},
		{
			name:     "uppercase config keys are not transcript",
			input:    "Host: localhost\nPort: 8080\nMode: production\n",
			wantKind: KindPlain,
			wantType: ComponentPreformatted,
		},
		{
			name:       "single timestamped severity log line",
			input:      "2026-06-16 12:00:00 ERROR stop\n",
			sourceName: "app.log",
			wantKind:   KindLog,
			wantType:   ComponentPreformatted,
		},
		{
			name:     "go test output beats tsv records",
			input:    "ok\tgithub.com/dotcommander/html/internal/cache\t0.012s\nok\tgithub.com/dotcommander/html/internal/render\t0.034s\n",
			wantKind: KindLog,
			wantType: ComponentPreformatted,
		},
		{
			name:       "tab file records",
			input:      "name\tscore\na\t1\nb\t2\n",
			sourceName: "scores.tab",
			wantKind:   KindTSVRecords,
			wantType:   ComponentDataTable,
		},
		{
			name:     "single go test package result is log",
			input:    "ok\tgithub.com/dotcommander/html/internal/cache\t0.012s\n",
			wantKind: KindLog,
			wantType: ComponentPreformatted,
		},
		{
			name:     "ordinary ok prose is plain",
			input:    "ok thanks\nok sure\n",
			wantKind: KindPlain,
			wantType: ComponentPreformatted,
		},
		{
			name:       "markdown with fenced code",
			input:      "# Title\n\n```go\nfmt.Println()\n```\n",
			sourceName: "doc.md",
			wantKind:   KindMarkdown,
			wantType:   ComponentArticle,
		},
		{
			name:       "markdown file with heading only",
			input:      "# Title\n\nBody text\n",
			sourceName: "doc.md",
			wantKind:   KindMarkdown,
			wantType:   ComponentArticle,
		},
		{
			name:       "markdown filename beats csv body",
			input:      "name,score\na,1\nb,2\n",
			sourceName: "doc.md",
			wantKind:   KindMarkdown,
			wantType:   ComponentArticle,
		},
		{
			name:       "markdown filename beats json body",
			input:      `[{"name":"a","score":1}]`,
			sourceName: "doc.markdown",
			wantKind:   KindMarkdown,
			wantType:   ComponentArticle,
		},
		{
			name:       "unknown extension with markdown structure",
			input:      "# Title\n\n```go\nfmt.Println()\n```\n",
			sourceName: "notes.mdish",
			wantKind:   KindMarkdown,
			wantType:   ComponentArticle,
		},
		{
			name:       "unknown extension with task list",
			input:      "- [x] ship renderer\n- [ ] verify screenshots\n",
			sourceName: "notes.mdish",
			wantKind:   KindMarkdown,
			wantType:   ComponentArticle,
		},
		{
			name:       "unknown extension with markdown heading",
			input:      "# Title\n\nBody text\n",
			sourceName: "notes.mdish",
			wantKind:   KindMarkdown,
			wantType:   ComponentArticle,
		},
		{
			name:       "unknown extension with setext h2 heading",
			input:      "Title\n-----\n\nBody text\n",
			sourceName: "notes.mdish",
			wantKind:   KindMarkdown,
			wantType:   ComponentArticle,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a, p := Plan(t.Context(), []byte(tt.input), Options{SourceName: tt.sourceName, Planner: PlannerOff})
			if a.Kind != tt.wantKind {
				t.Fatalf("kind = %s, want %s; reasons=%v", a.Kind, tt.wantKind, a.Reasons)
			}
			found := false
			for _, c := range p.Components {
				if c.Type == tt.wantType {
					found = true
				}
			}
			if !found {
				t.Fatalf("components = %#v, want type %s", p.Components, tt.wantType)
			}
		})
	}
}

func TestAnalyze_DelimitedPreservesRecordSpaces(t *testing.T) {
	t.Parallel()

	analysis := Analyze([]byte(" name,score\n Alpha,1 \n   \n"), "scores.csv")

	if analysis.Kind != KindCSVRecords {
		t.Fatalf("kind = %s, want %s; reasons=%v", analysis.Kind, KindCSVRecords, analysis.Reasons)
	}
	records, ok := analysis.Data.([][]string)
	if !ok {
		t.Fatalf("data type = %T, want [][]string", analysis.Data)
	}
	if got, want := records[0][0], " name"; got != want {
		t.Fatalf("header[0] = %q, want %q", got, want)
	}
	if got, want := records[1][0], " Alpha"; got != want {
		t.Fatalf("row[0] = %q, want %q", got, want)
	}
	if got, want := records[1][1], "1 "; got != want {
		t.Fatalf("row[1] = %q, want %q", got, want)
	}
}

func TestAnalyze_HeaderOnlyCSVDataFile(t *testing.T) {
	t.Parallel()

	src := []byte("name,score\n")
	analysis := Analyze(src, "empty.csv")

	if analysis.Kind != KindCSVRecords {
		t.Fatalf("kind = %s, want %s; reasons=%v", analysis.Kind, KindCSVRecords, analysis.Reasons)
	}
	if analysis.Stats.Records != 0 || analysis.Stats.Fields != 2 {
		t.Fatalf("stats = %#v, want 0 records and 2 fields", analysis.Stats)
	}
	records, ok := analysis.Data.([][]string)
	if !ok {
		t.Fatalf("data type = %T, want [][]string", analysis.Data)
	}
	if len(records) != 1 || len(records[0]) != 2 {
		t.Fatalf("records = %#v, want one header row with two fields", records)
	}
	if records[0][0] != "name" || records[0][1] != "score" {
		t.Fatalf("headers = %#v, want name/score", records[0])
	}
}

func TestAnalyze_ASCIITableRows(t *testing.T) {
	t.Parallel()

	src := []byte("+----+-------+\n| id | name  |\n+----+-------+\n| 1  | alpha |\n| 2  | beta  |\n+----+-------+\n")
	analysis := Analyze(src, "mysql.out")

	if analysis.Kind != KindTableRecords {
		t.Fatalf("kind = %s, want %s; reasons=%v", analysis.Kind, KindTableRecords, analysis.Reasons)
	}
	if analysis.Stats.Records != 2 || analysis.Stats.Fields != 2 {
		t.Fatalf("stats = %#v, want 2 records and 2 fields", analysis.Stats)
	}
	records, ok := analysis.Data.([][]string)
	if !ok {
		t.Fatalf("data = %T, want [][]string", analysis.Data)
	}
	if records[0][0] != "id" || records[0][1] != "name" || records[2][1] != "beta" {
		t.Fatalf("records = %#v, want parsed boxed table rows", records)
	}
}

func TestAnalyze_PlainDiffFileCountExcludesHunkContent(t *testing.T) {
	t.Parallel()

	src := []byte("--- old.txt\n+++ new.txt\n@@ -1,2 +1,2 @@\n-old\n+++ added heading\n")
	analysis := Analyze(src, "change.diff")

	if analysis.Kind != KindDiff {
		t.Fatalf("kind = %s, want %s; reasons=%v", analysis.Kind, KindDiff, analysis.Reasons)
	}
	if analysis.Stats.Files != 1 {
		t.Fatalf("files = %d, want 1", analysis.Stats.Files)
	}
}

func TestAnalyze_MixedReportsSignalNames(t *testing.T) {
	t.Parallel()

	src := []byte("Notes\n- check deploy\n\nPayload\n{\"ok\":true}\n\nERROR failed\n")
	analysis := Analyze(src, "")

	if analysis.Kind != KindMixed {
		t.Fatalf("kind = %s, want %s", analysis.Kind, KindMixed)
	}
	if len(analysis.Reasons) != 1 {
		t.Fatalf("reasons = %#v, want one reason", analysis.Reasons)
	}
	for _, want := range []string{"multiple weak format signals", "markdown-like prose", "json-like block", "log severity"} {
		if !strings.Contains(analysis.Reasons[0], want) {
			t.Fatalf("reason %q does not contain %q", analysis.Reasons[0], want)
		}
	}
}

func TestAnalyze_TreeStatsExcludeRootMarker(t *testing.T) {
	t.Parallel()

	analysis := Analyze([]byte(".\n|-- cmd\n|   `-- html\n`-- internal\n"), "tree.txt")

	if analysis.Kind != KindTreeListing {
		t.Fatalf("kind = %s, want %s; reasons=%v", analysis.Kind, KindTreeListing, analysis.Reasons)
	}
	if analysis.Stats.Files != 3 {
		t.Fatalf("files = %d, want 3 displayable entries", analysis.Stats.Files)
	}
}

func TestAnalyze_TreeStatsExcludeTreeSummary(t *testing.T) {
	t.Parallel()

	analysis := Analyze([]byte(".\n├── cmd\n│   └── html\n└── internal\n\n2 directories, 1 file\n"), "tree.txt")

	if analysis.Kind != KindTreeListing {
		t.Fatalf("kind = %s, want %s; reasons=%v", analysis.Kind, KindTreeListing, analysis.Reasons)
	}
	if analysis.Stats.Files != 3 {
		t.Fatalf("files = %d, want 3 displayable entries", analysis.Stats.Files)
	}
}

func TestAnalyze_TreeStatsExcludeDirectoryOnlyTreeSummary(t *testing.T) {
	t.Parallel()

	analysis := Analyze([]byte(".\n├── cmd\n│   └── html\n└── internal\n\n2 directories\n"), "tree.txt")

	if analysis.Kind != KindTreeListing {
		t.Fatalf("kind = %s, want %s; reasons=%v", analysis.Kind, KindTreeListing, analysis.Reasons)
	}
	if analysis.Stats.Files != 3 {
		t.Fatalf("files = %d, want 3 displayable entries", analysis.Stats.Files)
	}
}

func TestAnalyze_GoTestMarkersAtStartAreLogs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "leading fail marker",
			input: "--- FAIL: TestBroken (0.00s)\n    broken_test.go:12: boom\nFAIL\n",
		},
		{
			name:  "leading pass marker",
			input: "PASS\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			analysis := Analyze([]byte(tt.input), "")
			if analysis.Kind != KindLog {
				t.Fatalf("kind = %s, want %s; reasons=%v", analysis.Kind, KindLog, analysis.Reasons)
			}
		})
	}
}

func TestValidatePlanRejectsUnsafeLLMText(t *testing.T) {
	t.Parallel()
	p := ReportPlan{
		Version:    PlanVersion,
		Kind:       KindPlain,
		Layout:     LayoutSinglePage,
		Mode:       ModeBrief,
		Confidence: 0.5,
		Components: []Component{{Type: ComponentPreformatted, Source: "input", Title: "<script>alert(1)</script>", Options: map[string]string{}}},
	}
	if _, err := ValidatePlan(p); err == nil {
		t.Fatalf("expected unsafe title to be rejected")
	}
}

func TestValidatePlanRejectsBlankComponentTitle(t *testing.T) {
	t.Parallel()

	p := ReportPlan{
		Version:    PlanVersion,
		Kind:       KindPlain,
		Layout:     LayoutSinglePage,
		Mode:       ModeBrief,
		Confidence: 0.5,
		Components: []Component{{Type: ComponentPreformatted, Source: "input", Title: " \t", Options: map[string]string{}}},
	}
	if _, err := ValidatePlan(p); err == nil {
		t.Fatalf("expected blank component title to be rejected")
	}
}

func TestValidatePlanRejectsNonFiniteConfidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		confidence float64
	}{
		{name: "nan", confidence: math.NaN()},
		{name: "positive infinity", confidence: math.Inf(1)},
		{name: "negative infinity", confidence: math.Inf(-1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := ReportPlan{
				Version:    PlanVersion,
				Kind:       KindPlain,
				Layout:     LayoutSinglePage,
				Mode:       ModeBrief,
				Confidence: tt.confidence,
				Components: []Component{{Type: ComponentPreformatted, Source: "input", Title: "Input", Options: map[string]string{}}},
			}
			if _, err := ValidatePlan(p); err == nil {
				t.Fatalf("expected non-finite confidence to be rejected")
			}
		})
	}
}

func TestValidatePlanRejectsUnknownComponentSource(t *testing.T) {
	t.Parallel()

	p := ReportPlan{
		Version:    PlanVersion,
		Kind:       KindPlain,
		Layout:     LayoutSinglePage,
		Mode:       ModeBrief,
		Confidence: 0.5,
		Components: []Component{{Type: ComponentPreformatted, Source: "summary", Title: "Input", Options: map[string]string{}}},
	}
	if _, err := ValidatePlan(p); err == nil {
		t.Fatalf("expected unknown component source to be rejected")
	}
}

func TestValidatePlanRejectsComponentSourceMismatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		kind   Kind
		typ    ComponentType
		source string
	}{
		{name: "summary records", kind: KindCSVRecords, typ: ComponentSummary, source: "records"},
		{name: "table input", kind: KindCSVRecords, typ: ComponentDataTable, source: "input"},
		{name: "article records", kind: KindMarkdown, typ: ComponentArticle, source: "records"},
		{name: "raw json records", kind: KindJSONObject, typ: ComponentRawJSON, source: "records"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := ReportPlan{
				Version:    PlanVersion,
				Kind:       tt.kind,
				Layout:     LayoutSinglePage,
				Mode:       ModeBrief,
				Confidence: 0.5,
				Components: []Component{{Type: tt.typ, Source: tt.source, Title: "Input", Options: map[string]string{}}},
			}
			if _, err := ValidatePlan(p); err == nil {
				t.Fatalf("expected %s component with source %s to be rejected", tt.typ, tt.source)
			}
		})
	}
}

func TestValidatePlanRejectsUnrenderedLayoutComponents(t *testing.T) {
	t.Parallel()

	for _, typ := range []ComponentType{ComponentTabs, ComponentTOC} {
		t.Run(string(typ), func(t *testing.T) {
			t.Parallel()
			p := ReportPlan{
				Version:    PlanVersion,
				Kind:       KindPlain,
				Layout:     LayoutSinglePage,
				Mode:       ModeBrief,
				Confidence: 0.5,
				Components: []Component{{Type: typ, Source: "input", Title: "Input", Options: map[string]string{}}},
			}
			if _, err := ValidatePlan(p); err == nil {
				t.Fatalf("expected %s component to be rejected", typ)
			}
		})
	}
}

func TestValidatePlanRejectsKindIncompatibleComponents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind Kind
		typ  ComponentType
	}{
		{name: "plain table", kind: KindPlain, typ: ComponentDataTable},
		{name: "plain cards", kind: KindPlain, typ: ComponentRecordCards},
		{name: "json diff", kind: KindJSONObject, typ: ComponentDiffView},
		{name: "json tree", kind: KindJSONObject, typ: ComponentFileTree},
		{name: "plain raw json", kind: KindPlain, typ: ComponentRawJSON},
		{name: "source article", kind: KindSourceCode, typ: ComponentArticle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := ReportPlan{
				Version:    PlanVersion,
				Kind:       tt.kind,
				Layout:     LayoutSinglePage,
				Mode:       ModeBrief,
				Confidence: 0.5,
				Components: []Component{{Type: tt.typ, Source: "input", Title: "Input", Options: map[string]string{}}},
			}
			if _, err := ValidatePlan(p); err == nil {
				t.Fatalf("expected %s component to be rejected for %s", tt.typ, tt.kind)
			}
		})
	}
}

func TestPlan_TableOverrideRequiresRecordRows(t *testing.T) {
	t.Parallel()

	_, p := Plan(t.Context(), []byte("plain text\n"), Options{Mode: ModeOverrideTable, Planner: PlannerOff})

	if p.Kind != KindPlain {
		t.Fatalf("kind = %s, want %s", p.Kind, KindPlain)
	}
	if len(p.Components) != 2 || p.Components[0].Type != ComponentSummary || p.Components[1].Type != ComponentPreformatted {
		t.Fatalf("components = %#v, want summary and preformatted fallback", p.Components)
	}
}

func TestPlan_TableOverrideAppliesToCSVRows(t *testing.T) {
	t.Parallel()

	_, p := Plan(t.Context(), []byte("name,score\na,1\n"), Options{Mode: ModeOverrideTable, Planner: PlannerOff})

	if p.Kind != KindCSVRecords {
		t.Fatalf("kind = %s, want %s", p.Kind, KindCSVRecords)
	}
	if len(p.Components) != 1 || p.Components[0].Type != ComponentDataTable {
		t.Fatalf("components = %#v, want data table", p.Components)
	}
}

func TestPlan_SlidesLayoutOverride(t *testing.T) {
	t.Parallel()

	_, p := Plan(t.Context(), []byte("name,score\na,1\n"), Options{Layout: LayoutOverrideSlides, Planner: PlannerOff})

	if p.Layout != LayoutSlides {
		t.Fatalf("layout = %s, want %s", p.Layout, LayoutSlides)
	}
	if len(p.Components) == 0 {
		t.Fatalf("components is empty")
	}
}

func TestPlan_SourceCodeIncludesSummaryInAutoMode(t *testing.T) {
	t.Parallel()

	_, p := Plan(t.Context(), []byte("package main\n\nfunc main() {}\n"), Options{SourceName: "main.go", Planner: PlannerOff})

	if p.Kind != KindSourceCode {
		t.Fatalf("kind = %s, want %s", p.Kind, KindSourceCode)
	}
	if p.Mode != ModeCode {
		t.Fatalf("mode = %s, want %s", p.Mode, ModeCode)
	}
	if len(p.Components) != 2 {
		t.Fatalf("components = %#v, want summary and code", p.Components)
	}
	if p.Components[0].Type != ComponentSummary || p.Components[1].Type != ComponentCodeBlock {
		t.Fatalf("components = %#v, want summary then code", p.Components)
	}
}

func TestPlan_TranscriptUsesTranscriptTitle(t *testing.T) {
	t.Parallel()

	_, p := Plan(t.Context(), []byte("Host: Welcome back.\nGuest: Thanks for having me.\nHost: Let's begin.\n"), Options{Planner: PlannerOff})

	if p.Kind != KindTranscript {
		t.Fatalf("kind = %s, want %s", p.Kind, KindTranscript)
	}
	if len(p.Components) != 2 || p.Components[1].Title != "Transcript" {
		t.Fatalf("components = %#v, want Transcript preformatted component", p.Components)
	}
}

func TestPlan_PlainIncludesSummaryInAutoMode(t *testing.T) {
	t.Parallel()

	_, p := Plan(t.Context(), []byte("plain text\nanother line\n"), Options{Planner: PlannerOff})

	if p.Kind != KindPlain {
		t.Fatalf("kind = %s, want %s", p.Kind, KindPlain)
	}
	if len(p.Components) != 2 || p.Components[0].Type != ComponentSummary || p.Components[1].Type != ComponentPreformatted {
		t.Fatalf("components = %#v, want summary then input", p.Components)
	}
}

func TestPlan_CodeOverrideAppliesToCSVRows(t *testing.T) {
	t.Parallel()

	_, p := Plan(t.Context(), []byte("name,score\na,1\n"), Options{Mode: ModeOverrideCode, Planner: PlannerOff})

	if p.Kind != KindCSVRecords {
		t.Fatalf("kind = %s, want %s", p.Kind, KindCSVRecords)
	}
	if p.Mode != ModeCode {
		t.Fatalf("mode = %s, want %s", p.Mode, ModeCode)
	}
	if len(p.Components) != 1 || p.Components[0].Type != ComponentCodeBlock {
		t.Fatalf("components = %#v, want code block", p.Components)
	}
	for _, reason := range p.Reasons {
		if strings.Contains(reason, "deterministic plan invalid") {
			t.Fatalf("plan should not fall back after code override: %v", p.Reasons)
		}
	}
}

func TestPlan_CodeOverrideKeepsSourceCodePure(t *testing.T) {
	t.Parallel()

	_, p := Plan(t.Context(), []byte("package main\n\nfunc main() {}\n"), Options{SourceName: "main.go", Mode: ModeOverrideCode, Planner: PlannerOff})

	if p.Kind != KindSourceCode {
		t.Fatalf("kind = %s, want %s", p.Kind, KindSourceCode)
	}
	if len(p.Components) != 1 || p.Components[0].Type != ComponentCodeBlock {
		t.Fatalf("components = %#v, want code block only", p.Components)
	}
}

func TestPlan_PresentationOverridePreservesDetectedKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mode     ModeOverride
		wantMode Mode
		wantType ComponentType
	}{
		{name: "log", mode: ModeOverrideLog, wantMode: ModeConsole, wantType: ComponentPreformatted},
		{name: "code", mode: ModeOverrideCode, wantMode: ModeCode, wantType: ComponentCodeBlock},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, p := Plan(t.Context(), []byte("plain text\n"), Options{Mode: tt.mode, Planner: PlannerOff})
			if p.Kind != KindPlain {
				t.Fatalf("kind = %s, want %s", p.Kind, KindPlain)
			}
			if p.Mode != tt.wantMode {
				t.Fatalf("mode = %s, want %s", p.Mode, tt.wantMode)
			}
			if len(p.Components) != 1 || p.Components[0].Type != tt.wantType {
				t.Fatalf("components = %#v, want %s", p.Components, tt.wantType)
			}
		})
	}
}

func TestPlan_StructureOverrideRequiresMatchingKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode ModeOverride
	}{
		{name: "article", mode: ModeOverrideArticle},
		{name: "diff", mode: ModeOverrideDiff},
		{name: "tree", mode: ModeOverrideTree},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, p := Plan(t.Context(), []byte("plain text\n"), Options{Mode: tt.mode, Planner: PlannerOff})
			if p.Kind != KindPlain {
				t.Fatalf("kind = %s, want %s", p.Kind, KindPlain)
			}
			if len(p.Components) != 2 || p.Components[0].Type != ComponentSummary || p.Components[1].Type != ComponentPreformatted {
				t.Fatalf("components = %#v, want summary and preformatted fallback", p.Components)
			}
		})
	}
}

func TestPlan_StructureOverrideAppliesToMatchingKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		mode     ModeOverride
		wantKind Kind
		wantType ComponentType
	}{
		{
			name:     "article",
			input:    "# Title\n\n```go\nfmt.Println()\n```\n",
			mode:     ModeOverrideArticle,
			wantKind: KindMarkdown,
			wantType: ComponentArticle,
		},
		{
			name:     "diff",
			input:    "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-a\n+b\n",
			mode:     ModeOverrideDiff,
			wantKind: KindDiff,
			wantType: ComponentDiffView,
		},
		{
			name:     "tree",
			input:    ".\n├── cmd\n└── internal\n",
			mode:     ModeOverrideTree,
			wantKind: KindTreeListing,
			wantType: ComponentFileTree,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, p := Plan(t.Context(), []byte(tt.input), Options{Mode: tt.mode, Planner: PlannerOff})
			if p.Kind != tt.wantKind {
				t.Fatalf("kind = %s, want %s", p.Kind, tt.wantKind)
			}
			if len(p.Components) != 1 || p.Components[0].Type != tt.wantType {
				t.Fatalf("components = %#v, want %s", p.Components, tt.wantType)
			}
		})
	}
}

func TestLLMCacheKeyIncludesReportOverrides(t *testing.T) {
	t.Parallel()

	src := []byte("name,score\nAlpha,10\n")
	analysis := Analyze(src, "scores.csv")
	base := Options{Planner: PlannerLLM, LLMModel: "test-model", Mode: ModeOverrideAuto, Layout: LayoutOverrideAuto}

	autoKey, _ := llmCacheKey(src, analysis, base)
	tableKey, _ := llmCacheKey(src, analysis, Options{Planner: PlannerLLM, LLMModel: "test-model", Mode: ModeOverrideTable, Layout: LayoutOverrideAuto})
	tabsKey, _ := llmCacheKey(src, analysis, Options{Planner: PlannerLLM, LLMModel: "test-model", Mode: ModeOverrideAuto, Layout: LayoutOverrideTabs})
	slidesKey, _ := llmCacheKey(src, analysis, Options{Planner: PlannerLLM, LLMModel: "test-model", Mode: ModeOverrideAuto, Layout: LayoutOverrideSlides})

	if autoKey == tableKey {
		t.Fatalf("mode override must affect LLM planner cache key")
	}
	if autoKey == tabsKey {
		t.Fatalf("layout override must affect LLM planner cache key")
	}
	if autoKey == slidesKey || tabsKey == slidesKey {
		t.Fatalf("slides layout override must have a distinct LLM planner cache key")
	}
}

func TestLLMUserPromptAllowsSlidesLayout(t *testing.T) {
	t.Parallel()

	src := []byte("name,score\nAlpha,10\n")
	analysis := Analyze(src, "scores.csv")
	fallback := deterministicPlan(analysis, Options{})
	_, summary := llmCacheKey(src, analysis, Options{Planner: PlannerLLM, LLMModel: "test-model"})
	prompt := llmUserPrompt(analysis, fallback, summary, src)

	if !strings.Contains(prompt, string(LayoutSlides)) {
		t.Fatalf("planner prompt does not mention %s", LayoutSlides)
	}
}

func TestLLMCacheKeyIncludesPlannerEndpoint(t *testing.T) {
	t.Parallel()

	src := []byte("plain text\nanother line\n")
	analysis := Analyze(src, "")
	base := Options{
		Planner:  PlannerLLM,
		LLMModel: "test-model",
		LLMURL:   "http://127.0.0.1:1/v1/chat/completions",
	}

	firstKey, _ := llmCacheKey(src, analysis, base)
	second := base
	second.LLMURL = "http://127.0.0.1:2/v1/chat/completions"
	secondKey, _ := llmCacheKey(src, analysis, second)

	if firstKey == secondKey {
		t.Fatalf("LLM planner endpoint must affect cache key")
	}
}

func TestPlan_RejectsLLMComponentsIncompatibleWithAnalysis(t *testing.T) {
	t.Parallel()

	llmPlan := ReportPlan{
		Version:    PlanVersion,
		Kind:       KindCSVRecords,
		Layout:     LayoutSinglePage,
		Mode:       ModeDataBrowser,
		Confidence: 0.9,
		Components: []Component{{Type: ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}}},
		Planner:    PlannerInfo{Name: "llm", Contributed: true},
	}
	planBytes, err := json.Marshal(llmPlan)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Cleanup(func() { _ = r.Body.Close() })
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": string(planBytes)}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	analysis, p := Plan(t.Context(), []byte("plain text\nanother line\n"), Options{
		Planner:    PlannerLLM,
		LLMURL:     srv.URL,
		LLMModel:   "test-model",
		LLMTimeout: "2s",
	})

	if analysis.Kind != KindPlain {
		t.Fatalf("analysis kind = %s, want %s", analysis.Kind, KindPlain)
	}
	if p.Kind != KindPlain {
		t.Fatalf("plan kind = %s, want deterministic fallback kind %s", p.Kind, KindPlain)
	}
	if len(p.Components) != 2 || p.Components[0].Type != ComponentSummary || p.Components[1].Type != ComponentPreformatted {
		t.Fatalf("components = %#v, want deterministic plain fallback", p.Components)
	}
	foundReason := false
	for _, reason := range p.Reasons {
		if strings.Contains(reason, "incompatible with analysis kind") {
			foundReason = true
		}
	}
	if !foundReason {
		t.Fatalf("fallback reason should explain analysis incompatibility, got %v", p.Reasons)
	}
}

func TestPlan_RejectsLLMBlankComponentTitle(t *testing.T) {
	t.Parallel()

	llmPlan := ReportPlan{
		Version:    PlanVersion,
		Kind:       KindPlain,
		Layout:     LayoutSinglePage,
		Mode:       ModeBrief,
		Confidence: 0.9,
		Components: []Component{{Type: ComponentPreformatted, Source: "input", Title: " \t", Options: map[string]string{}}},
		Planner:    PlannerInfo{Name: "llm", Contributed: true},
	}
	planBytes, err := json.Marshal(llmPlan)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Cleanup(func() { _ = r.Body.Close() })
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": string(planBytes)}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	analysis, p := Plan(t.Context(), []byte("plain text\nanother line\n"), Options{
		Planner:    PlannerLLM,
		LLMURL:     srv.URL,
		LLMModel:   "test-model",
		LLMTimeout: "2s",
	})

	if analysis.Kind != KindPlain {
		t.Fatalf("analysis kind = %s, want %s", analysis.Kind, KindPlain)
	}
	if p.Kind != KindPlain {
		t.Fatalf("plan kind = %s, want deterministic fallback kind %s", p.Kind, KindPlain)
	}
	if len(p.Components) != 2 || p.Components[0].Type != ComponentSummary || p.Components[1].Type != ComponentPreformatted {
		t.Fatalf("components = %#v, want deterministic plain fallback", p.Components)
	}
	if strings.TrimSpace(p.Components[1].Title) == "" {
		t.Fatalf("fallback component title is blank: %#v", p.Components[1])
	}
	foundReason := false
	for _, reason := range p.Reasons {
		if strings.Contains(reason, "blank title") {
			foundReason = true
		}
	}
	if !foundReason {
		t.Fatalf("fallback reason should explain blank component title, got %v", p.Reasons)
	}
}

func TestPlan_LLMHTTPErrorReportsStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "planner unavailable", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	_, p := Plan(t.Context(), []byte("plain text\nanother line\n"), Options{
		Planner:    PlannerLLM,
		LLMURL:     srv.URL,
		LLMModel:   "test-model",
		LLMTimeout: "2s",
	})

	foundReason := false
	for _, reason := range p.Reasons {
		if strings.Contains(reason, "llm planner returned status 500") {
			foundReason = true
		}
		if strings.Contains(reason, "invalid response") {
			t.Fatalf("http status errors should not be reported as JSON decode failures: %v", p.Reasons)
		}
	}
	if !foundReason {
		t.Fatalf("fallback reason should report HTTP status, got %v", p.Reasons)
	}
}

func TestPlan_LLMEmptyChoicesReportsResponseShape(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
	}))
	t.Cleanup(srv.Close)

	_, p := Plan(t.Context(), []byte("plain text\nanother line\n"), Options{
		Planner:    PlannerLLM,
		LLMURL:     srv.URL,
		LLMModel:   "test-model",
		LLMTimeout: "2s",
	})

	foundReason := false
	for _, reason := range p.Reasons {
		if strings.Contains(reason, "llm planner returned no choices") {
			foundReason = true
		}
		if strings.Contains(reason, "returned status 200") {
			t.Fatalf("empty choices should not be reported as HTTP status success: %v", p.Reasons)
		}
	}
	if !foundReason {
		t.Fatalf("fallback reason should report empty choices, got %v", p.Reasons)
	}
}

func TestPlan_LLMInvalidTimeoutDoesNotCallEndpoint(t *testing.T) {
	t.Parallel()

	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "should not be called", http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	_, p := Plan(t.Context(), []byte("plain text\nanother line\n"), Options{
		Planner:    PlannerLLM,
		LLMURL:     srv.URL,
		LLMModel:   "test-model",
		LLMTimeout: "not-a-duration",
	})

	if requests != 0 {
		t.Fatalf("invalid timeout should prevent planner request, got %d requests", requests)
	}
	foundReason := false
	for _, reason := range p.Reasons {
		if strings.Contains(reason, "llm planner invalid timeout") {
			foundReason = true
		}
		if strings.Contains(reason, "returned status") {
			t.Fatalf("invalid timeout should not be reported as endpoint failure: %v", p.Reasons)
		}
	}
	if !foundReason {
		t.Fatalf("fallback reason should report invalid timeout, got %v", p.Reasons)
	}
}

func TestPlan_LLMInvalidTimeoutBypassesCachedPlan(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	src := []byte("plain text\nanother line\n")
	llmPlan := ReportPlan{
		Version:    PlanVersion,
		Kind:       KindPlain,
		Layout:     LayoutSinglePage,
		Mode:       ModeBrief,
		Confidence: 0.9,
		Components: []Component{{Type: ComponentPreformatted, Source: "input", Title: "LLM Input", Options: map[string]string{}}},
	}
	planBytes, err := json.Marshal(llmPlan)
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": string(planBytes)}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	opts := Options{
		Planner:    PlannerLLM,
		LLMURL:     srv.URL,
		LLMModel:   "test-model",
		LLMTimeout: "2s",
	}
	_, cached := Plan(t.Context(), src, opts)
	if requests != 1 {
		t.Fatalf("expected one planner request to seed cache, got %d", requests)
	}
	if len(cached.Components) != 1 || cached.Components[0].Title != "LLM Input" || cached.Planner.Cache != "miss" {
		t.Fatalf("expected seeded LLM plan, got %#v", cached)
	}

	opts.LLMTimeout = "not-a-duration"
	_, p := Plan(t.Context(), src, opts)
	if requests != 1 {
		t.Fatalf("invalid timeout should not call endpoint after cache seed, got %d requests", requests)
	}
	if p.Planner.Name != "deterministic" {
		t.Fatalf("invalid timeout should use deterministic fallback, got planner %#v", p.Planner)
	}
	if len(p.Components) == 1 && p.Components[0].Title == "LLM Input" {
		t.Fatalf("invalid timeout should not use cached LLM plan: %#v", p.Components)
	}
	foundReason := false
	for _, reason := range p.Reasons {
		if strings.Contains(reason, "llm planner invalid timeout") {
			foundReason = true
		}
	}
	if !foundReason {
		t.Fatalf("fallback reason should report invalid timeout, got %v", p.Reasons)
	}
}
