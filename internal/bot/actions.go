package bot

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	telebot "gopkg.in/telebot.v3"

	"github.com/ikkairat/telegram-checklist-bot/internal/domain"
	"github.com/ikkairat/telegram-checklist-bot/internal/pdf"
	"github.com/ikkairat/telegram-checklist-bot/internal/service"
)

// These action methods hold the actual business steps behind both the
// manual admin commands and the scheduler's cron jobs, so the two never
// drift apart. Each returns a human-readable outcome string suitable for
// replying to an admin or logging from the scheduler.

// PostChecklist sends today's checklist to the group, unless it was
// already sent or there are no active tasks.
func (b *Bot) PostChecklist(ctx context.Context) (string, error) {
	today := b.Now()

	existing, err := b.svc.GetPoll(ctx, today)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return "Чек-лист на сегодня уже был отправлен в группу.", nil
	}

	tasks, err := b.svc.ActiveTasks(ctx)
	if err != nil {
		return "", err
	}
	if len(tasks) == 0 {
		return "Нет активных задач — сначала добавь их через /add.", nil
	}

	statuses, err := b.svc.BuildStatuses(ctx, today)
	if err != nil {
		return "", err
	}

	msg, err := b.Send(telebot.ChatID(b.cfg.GroupChatID),
		BuildChecklistText(today, tasks, statuses), BuildChecklistKeyboard(tasks, today))
	if err != nil {
		return "", fmt.Errorf("send checklist to group: %w", err)
	}

	if _, err := b.svc.CreatePoll(ctx, today, int64(msg.ID)); err != nil {
		return "", err
	}
	return "📋 Чек-лист отправлен в группу.", nil
}

// SendReminders DMs every registered user who still has unchecked tasks
// for today. Users who never opened a private chat with the bot are
// silently skipped (counted, not failed) — see architecture doc §7.
func (b *Bot) SendReminders(ctx context.Context) (string, error) {
	today := b.Now()

	poll, err := b.svc.GetPoll(ctx, today)
	if err != nil {
		return "", err
	}
	if poll == nil {
		return "Чек-лист на сегодня ещё не отправлен — напоминать не о чем.", nil
	}
	if poll.IsClosed {
		return "День уже закрыт.", nil
	}

	tasks, err := b.svc.ActiveTasks(ctx)
	if err != nil {
		return "", err
	}
	statuses, err := b.svc.BuildStatuses(ctx, today)
	if err != nil {
		return "", err
	}

	sent, skipped := 0, 0
	for _, st := range statuses {
		var missing []string
		for _, t := range tasks {
			if !st.Done[t.ID] {
				missing = append(missing, t.Title)
			}
		}
		if len(missing) == 0 {
			continue
		}

		text := fmt.Sprintf("⏰ Напоминание: сегодня (%s) у тебя ещё не отмечено:\n- %s",
			today.Format("02.01.2006"), strings.Join(missing, "\n- "))
		if _, err := b.Send(telebot.ChatID(st.User.TelegramID), text); err != nil {
			skipped++
			continue
		}
		sent++
	}

	return fmt.Sprintf("Напоминания разосланы: %d, пропущено (нет диалога с ботом): %d.", sent, skipped), nil
}

// CloseDay marks today's poll closed. Unchecked tasks simply stay absent
// from checkins and are treated as missed everywhere they're counted.
func (b *Bot) CloseDay(ctx context.Context) (string, error) {
	today := b.Now()

	poll, err := b.svc.GetPoll(ctx, today)
	if err != nil {
		return "", err
	}
	if poll == nil {
		return "Чек-лист на сегодня не был отправлен — закрывать нечего.", nil
	}
	if poll.IsClosed {
		return "День уже закрыт.", nil
	}

	if err := b.svc.ClosePoll(ctx, poll.ID); err != nil {
		return "", err
	}
	return "🔒 День закрыт, неотмеченные пункты зафиксированы как невыполненные.", nil
}

// PostWeeklyReport closes the current open week (computing penalties and
// adding them to the fund), announces the results in the group and to
// admins, and opens the next week with the same penalty rate.
func (b *Bot) PostWeeklyReport(ctx context.Context) (string, error) {
	week, err := b.penaltySvc.GetOpenWeek(ctx)
	if err != nil {
		return "", err
	}
	if week == nil {
		return "Ставка штрафа ещё не задана — используйте /setpenalty, чтобы начать первую неделю.", nil
	}

	results, weekFundTotal, err := b.penaltySvc.CloseWeek(ctx, week)
	if err != nil {
		return "", err
	}
	grandTotal, err := b.penaltySvc.FundTotal(ctx)
	if err != nil {
		return "", err
	}

	outcome, err := b.publishWeeklyReport(ctx, week, results, weekFundTotal, grandTotal)
	if err != nil {
		return "", err
	}

	if _, err := b.penaltySvc.StartNextWeek(ctx, week); err != nil {
		return "", fmt.Errorf("start next week: %w", err)
	}

	return outcome, nil
}

// publishWeeklyReport generates the PDF report and sends it (as a document)
// to the group and every admin. If PDF generation fails, it falls back to
// a plain-text summary so the computed numbers aren't lost — the penalties
// are already persisted by this point regardless.
func (b *Bot) publishWeeklyReport(ctx context.Context, week *domain.Week, results []service.UserPenalty, weekFundTotal, grandTotal int64) (string, error) {
	caption := fmt.Sprintf("🧾 Отчёт за неделю %s — %s\nШтрафов начислено: %d сом\nОбщий фонд: %d сом",
		week.StartDate.Format("02.01.2006"), week.EndDate.Format("02.01.2006"), weekFundTotal, grandTotal)

	pdfPath := filepath.Join(b.cfg.ReportsDir, fmt.Sprintf("week_%s.pdf", week.EndDate.Format("2006-01-02")))
	genErr := pdf.GenerateWeeklyReport(week, results, weekFundTotal, grandTotal, pdfPath)
	if genErr != nil {
		text := buildWeeklyReportText(week, results, weekFundTotal, grandTotal)
		if _, err := b.Send(telebot.ChatID(b.cfg.GroupChatID), text); err != nil {
			return "", fmt.Errorf("post weekly report to group: %w", err)
		}
		for _, adminID := range b.cfg.AdminTelegramIDs {
			_, _ = b.Send(telebot.ChatID(adminID), text)
		}
		return fmt.Sprintf("%s\n\n⚠️ PDF не сформирован: %v", text, genErr), nil
	}

	if err := b.penaltySvc.SetReportPath(ctx, week.ID, pdfPath); err != nil {
		return "", err
	}

	doc := &telebot.Document{
		File:     telebot.FromDisk(pdfPath),
		FileName: filepath.Base(pdfPath),
		Caption:  caption,
	}
	if _, err := b.Send(telebot.ChatID(b.cfg.GroupChatID), doc); err != nil {
		return "", fmt.Errorf("post weekly report pdf to group: %w", err)
	}
	for _, adminID := range b.cfg.AdminTelegramIDs {
		_, _ = b.Send(telebot.ChatID(adminID), doc)
	}

	return caption, nil
}

func buildWeeklyReportText(week *domain.Week, results []service.UserPenalty, weekFundTotal, grandTotal int64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🧾 Итоги недели %s — %s\n\n",
		week.StartDate.Format("02.01.2006"), week.EndDate.Format("02.01.2006"))

	for _, r := range results {
		fmt.Fprintf(&b, "%s: пропущено %d/%d — %d сом\n",
			r.User.FullName, r.Penalty.MissedTasks, r.Penalty.TotalTasks, r.Penalty.Amount)
	}

	fmt.Fprintf(&b, "\nШтрафов начислено за неделю: %d сом\nОбщий фонд: %d сом", weekFundTotal, grandTotal)
	return b.String()
}
