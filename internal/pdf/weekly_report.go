package pdf

import (
	"fmt"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/props"

	"github.com/ikkairat/telegram-checklist-bot/internal/domain"
	"github.com/ikkairat/telegram-checklist-bot/internal/service"
)

// GenerateWeeklyReport renders a one-page PDF summary of a week's checklist
// penalties (participant × completed/missed/amount, plus fund totals) and
// saves it to path.
func GenerateWeeklyReport(week *domain.Week, results []service.UserPenalty, weekFundTotal, grandTotal int64, path string) error {
	cfg := config.NewBuilder().WithPageNumber().Build()
	m := maroto.New(cfg)

	m.AddRow(16, text.NewCol(12, "Отчёт по штрафам чек-листа", props.Text{
		Size: 16, Style: fontstyle.Bold, Align: align.Center,
	}))
	m.AddRow(10, text.NewCol(12,
		fmt.Sprintf("Неделя: %s — %s", week.StartDate.Format("02.01.2006"), week.EndDate.Format("02.01.2006")),
		props.Text{Size: 11, Align: align.Center}))
	m.AddRow(8, text.NewCol(12,
		fmt.Sprintf("Ставка за пропуск: %d сом", week.PenaltyAmount),
		props.Text{Size: 10, Align: align.Center}))

	m.AddRow(6)

	m.AddRow(8,
		text.NewCol(5, "Участник", props.Text{Style: fontstyle.Bold}),
		text.NewCol(2, "Выполнено", props.Text{Style: fontstyle.Bold, Align: align.Center}),
		text.NewCol(2, "Пропущено", props.Text{Style: fontstyle.Bold, Align: align.Center}),
		text.NewCol(2, "Штраф", props.Text{Style: fontstyle.Bold, Align: align.Right}),
		text.NewCol(1, "Опл.", props.Text{Style: fontstyle.Bold, Align: align.Center}),
	)

	for _, r := range results {
		paid := "—"
		if r.Penalty.IsPaid {
			paid = "✓"
		}
		completed := r.Penalty.TotalTasks - r.Penalty.MissedTasks

		m.AddRow(7,
			text.NewCol(5, r.User.FullName),
			text.NewCol(2, fmt.Sprintf("%d/%d", completed, r.Penalty.TotalTasks), props.Text{Align: align.Center}),
			text.NewCol(2, fmt.Sprintf("%d", r.Penalty.MissedTasks), props.Text{Align: align.Center}),
			text.NewCol(2, fmt.Sprintf("%d", r.Penalty.Amount), props.Text{Align: align.Right}),
			text.NewCol(1, paid, props.Text{Align: align.Center}),
		)
	}

	m.AddRow(10)
	m.AddRow(8, text.NewCol(12,
		fmt.Sprintf("Штрафов начислено за неделю: %d сом", weekFundTotal), props.Text{Style: fontstyle.Bold}))
	m.AddRow(8, text.NewCol(12,
		fmt.Sprintf("Общий фонд: %d сом", grandTotal), props.Text{Style: fontstyle.Bold}))

	doc, err := m.Generate()
	if err != nil {
		return fmt.Errorf("generate weekly report pdf: %w", err)
	}
	if err := doc.Save(path); err != nil {
		return fmt.Errorf("save weekly report pdf to %s: %w", path, err)
	}
	return nil
}
