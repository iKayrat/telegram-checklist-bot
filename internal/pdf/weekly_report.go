package pdf

import (
	"fmt"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/image"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/border"
	"github.com/johnfercher/maroto/v2/pkg/consts/extension"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/core/entity"
	"github.com/johnfercher/maroto/v2/pkg/props"

	"github.com/ikkairat/telegram-checklist-bot/internal/domain"
	"github.com/ikkairat/telegram-checklist-bot/internal/service"
)

// Color palette for the "official-looking" report styling: a dark navy
// header/border and light gray for alternating row shading. The stamp
// (stamp.go) uses a separate muted maroon so it reads as a seal accent
// rather than blending into the table.
var (
	colorHeaderBg = &props.Color{Red: 33, Green: 47, Blue: 61}
	colorRowAltBg = &props.Color{Red: 245, Green: 245, Blue: 245}
	colorWhite    = &props.Color{Red: 255, Green: 255, Blue: 255}
	colorBorder   = &props.Color{Red: 33, Green: 47, Blue: 61}
)

// GenerateWeeklyReport renders a one-page PDF summary of a week's checklist
// penalties (participant × completed/missed/amount, plus fund totals) and
// saves it to path.
func GenerateWeeklyReport(week *domain.Week, results []service.UserPenalty, weekFundTotal, grandTotal int64, path string) error {
	cfg := config.NewBuilder().
		WithPageNumber().
		WithCustomFonts([]*entity.CustomFont{
			{Family: reportFontFamily, Style: fontstyle.Normal, Bytes: fontRegular},
			{Family: reportFontFamily, Style: fontstyle.Bold, Bytes: fontBold},
		}).
		WithDefaultFont(&props.Font{Family: reportFontFamily}).
		Build()
	m := maroto.New(cfg)

	if stampPNG, err := GenerateStamp(); err == nil {
		footerRow := row.New(45).Add(
			text.NewCol(8, ""),
			image.NewFromBytesCol(4, stampPNG, extension.Png, props.Rect{Center: true, Percent: 90}),
		)
		// A broken/missing stamp shouldn't break the whole report — the
		// financial numbers matter far more than the decorative seal.
		_ = m.RegisterFooter(footerRow)
	}

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

	headerRow := row.New(9).Add(
		text.NewCol(5, "Участник", props.Text{Style: fontstyle.Bold, Color: colorWhite}),
		text.NewCol(2, "Выполнено", props.Text{Style: fontstyle.Bold, Align: align.Center, Color: colorWhite}),
		text.NewCol(2, "Пропущено", props.Text{Style: fontstyle.Bold, Align: align.Center, Color: colorWhite}),
		text.NewCol(2, "Штраф", props.Text{Style: fontstyle.Bold, Align: align.Right, Color: colorWhite}),
		text.NewCol(1, "Опл.", props.Text{Style: fontstyle.Bold, Align: align.Center, Color: colorWhite}),
	).WithStyle(&props.Cell{BackgroundColor: colorHeaderBg})
	m.AddRows(headerRow)

	for i, r := range results {
		paid := "—"
		if r.Penalty.IsPaid {
			paid = "✓"
		}
		completed := r.Penalty.TotalTasks - r.Penalty.MissedTasks

		dataRow := row.New(7).Add(
			text.NewCol(5, r.User.FullName),
			text.NewCol(2, fmt.Sprintf("%d/%d", completed, r.Penalty.TotalTasks), props.Text{Align: align.Center}),
			text.NewCol(2, fmt.Sprintf("%d", r.Penalty.MissedTasks), props.Text{Align: align.Center}),
			text.NewCol(2, fmt.Sprintf("%d", r.Penalty.Amount), props.Text{Align: align.Right}),
			text.NewCol(1, paid, props.Text{Align: align.Center}),
		)
		if i%2 == 1 {
			dataRow = dataRow.WithStyle(&props.Cell{BackgroundColor: colorRowAltBg})
		}
		m.AddRows(dataRow)
	}

	m.AddRow(10)

	// Two rows sharing a background, with complementary border masks (top row
	// gets no bottom border, bottom row gets no top border) so together they
	// read as a single boxed summary block.
	summaryBoxStyle := &props.Cell{BackgroundColor: colorRowAltBg, BorderColor: colorBorder, BorderThickness: 0.5}
	summaryLine1 := row.New(12).Add(
		text.NewCol(12, fmt.Sprintf("Штрафов начислено за неделю: %d сом", weekFundTotal),
			props.Text{Style: fontstyle.Bold, Top: 3, Left: 3}),
	).WithStyle(&props.Cell{
		BackgroundColor: summaryBoxStyle.BackgroundColor,
		BorderColor:     summaryBoxStyle.BorderColor,
		BorderThickness: summaryBoxStyle.BorderThickness,
		BorderType:      border.Left | border.Top | border.Right,
	})
	summaryLine2 := row.New(12).Add(
		text.NewCol(12, fmt.Sprintf("Общий фонд: %d сом", grandTotal),
			props.Text{Style: fontstyle.Bold, Top: 1, Left: 3}),
	).WithStyle(&props.Cell{
		BackgroundColor: summaryBoxStyle.BackgroundColor,
		BorderColor:     summaryBoxStyle.BorderColor,
		BorderThickness: summaryBoxStyle.BorderThickness,
		BorderType:      border.Left | border.Bottom | border.Right,
	})
	m.AddRows(summaryLine1, summaryLine2)

	doc, err := m.Generate()
	if err != nil {
		return fmt.Errorf("generate weekly report pdf: %w", err)
	}
	if err := doc.Save(path); err != nil {
		return fmt.Errorf("save weekly report pdf to %s: %w", path, err)
	}
	return nil
}
