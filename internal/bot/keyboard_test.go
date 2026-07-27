package bot

import (
	"strings"
	"testing"

	"github.com/ikkairat/telegram-checklist-bot/internal/domain"
)

func TestRenderTaskTable(t *testing.T) {
	tasks := []domain.Task{
		{ID: 1, Title: "Прочитать книгу"},
		{ID: 12, Title: "Спорт"},
	}

	got := renderTaskTable(tasks)

	if !strings.HasPrefix(got, "<pre>\n") || !strings.HasSuffix(got, "</pre>") {
		t.Fatalf("expected table wrapped in <pre>...</pre>, got: %s", got)
	}

	lines := strings.Split(strings.TrimSuffix(strings.TrimPrefix(got, "<pre>\n"), "</pre>"), "\n")
	// header, separator, row for id=1, row for id=12, trailing empty line
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %q", len(lines), lines)
	}

	header, sep, row1, row12 := lines[0], lines[1], lines[2], lines[3]
	if !strings.HasPrefix(header, "ID") || !strings.Contains(header, "Задача") {
		t.Fatalf("unexpected header: %q", header)
	}
	if !strings.HasPrefix(sep, "--") {
		t.Fatalf("unexpected separator: %q", sep)
	}

	// the ID column must be as wide as the widest id ("12"), so both rows'
	// second column (the task title) must start at the same offset.
	idColWidth := strings.Index(row1, "Прочитать")
	if idColWidth != strings.Index(row12, "Спорт") {
		t.Fatalf("columns not aligned:\nrow1:  %q\nrow12: %q", row1, row12)
	}
}

func TestRenderTaskTableEscapesHTML(t *testing.T) {
	tasks := []domain.Task{{ID: 1, Title: "<script>alert(1)</script> & co"}}

	got := renderTaskTable(tasks)

	if strings.Contains(got, "<script>") {
		t.Fatalf("expected task title to be HTML-escaped, got: %s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") || !strings.Contains(got, "&amp; co") {
		t.Fatalf("expected escaped entities in output: %s", got)
	}
}
