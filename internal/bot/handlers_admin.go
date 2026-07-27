package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"unicode/utf8"

	telebot "gopkg.in/telebot.v3"

	"github.com/ikkairat/telegram-checklist-bot/internal/domain"
	"github.com/ikkairat/telegram-checklist-bot/internal/service"
)

func (b *Bot) registerAdminHandlers() {
	b.Handle("/add", b.handleAddTask, b.adminOnly)
	b.Handle("/remove", b.handleRemoveTask, b.adminOnly)
	b.Handle("/list", b.handleListTasks, b.adminOnly)
	b.Handle("/post_checklist", b.handlePostChecklist, b.adminOnly)
	b.Handle("/post_report", b.handlePostReport, b.adminOnly)
	b.Handle("/setpenalty", b.handleSetPenalty, b.adminOnly)
	b.Handle("/mark_paid", b.handleMarkPaid, b.adminOnly)
	b.Handle("/forgive", b.handleForgive, b.adminOnly)
	b.Handle("/setadmin", b.handleSetAdmin, b.adminOnly)
	b.Handle("/unsetadmin", b.handleUnsetAdmin, b.adminOnly)
}

func (b *Bot) handleAddTask(c telebot.Context) error {
	title := strings.TrimSpace(c.Data())
	if title == "" {
		return c.Reply("Использование: /add <название задачи>")
	}

	task, err := b.svc.AddTask(context.Background(), title)
	if err != nil {
		return err
	}
	return c.Reply(fmt.Sprintf("✅ Задача добавлена: %s (ID %d)", task.Title, task.ID))
}

func (b *Bot) handleRemoveTask(c telebot.Context) error {
	idStr := strings.TrimSpace(c.Data())
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Reply("Использование: /remove <ID задачи>")
	}

	if err := b.svc.RemoveTask(context.Background(), id); err != nil {
		return err
	}
	return c.Reply("🗑 Задача удалена из чек-листа.")
}

func (b *Bot) handleListTasks(c telebot.Context) error {
	tasks, err := b.svc.ActiveTasks(context.Background())
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return c.Reply("Активных задач нет. Добавь через /add.")
	}

	return c.Reply(renderTaskTable(tasks), telebot.ModeHTML)
}

// renderTaskTable formats tasks as a monospace table (Telegram has no
// native table rendering, so a <pre> block with padded columns is the
// standard way to fake one).
func renderTaskTable(tasks []domain.Task) string {
	idWidth := utf8.RuneCountInString("ID")
	titleWidth := utf8.RuneCountInString("Задача")
	for _, t := range tasks {
		if w := utf8.RuneCountInString(strconv.FormatInt(t.ID, 10)); w > idWidth {
			idWidth = w
		}
		if w := utf8.RuneCountInString(t.Title); w > titleWidth {
			titleWidth = w
		}
	}

	var sb strings.Builder
	sb.WriteString("<pre>\n")
	fmt.Fprintf(&sb, "%-*s  %s\n", idWidth, "ID", "Задача")
	fmt.Fprintf(&sb, "%s  %s\n", strings.Repeat("-", idWidth), strings.Repeat("-", titleWidth))
	for _, t := range tasks {
		fmt.Fprintf(&sb, "%-*d  %s\n", idWidth, t.ID, html.EscapeString(t.Title))
	}
	sb.WriteString("</pre>")
	return sb.String()
}

// handlePostChecklist posts today's checklist by hand; internal/scheduler
// calls b.PostChecklist on the same cron job instead.
func (b *Bot) handlePostChecklist(c telebot.Context) error {
	text, err := b.PostChecklist(context.Background())
	if err != nil {
		return err
	}
	return c.Reply(text)
}

// handlePostReport closes the current open week and sends the PDF report by
// hand; internal/scheduler calls b.PostWeeklyReport on the same cron job
// instead. This is the real thing, not a preview — it closes the week for
// good, same as the scheduled run.
func (b *Bot) handlePostReport(c telebot.Context) error {
	text, err := b.PostWeeklyReport(context.Background())
	if err != nil {
		return err
	}
	return c.Reply(text)
}

func (b *Bot) handleSetPenalty(c telebot.Context) error {
	amount, err := strconv.ParseInt(strings.TrimSpace(c.Data()), 10, 64)
	if err != nil || amount < 0 {
		return c.Reply("Использование: /setpenalty <сумма за один пропуск, сом>")
	}

	week, err := b.penaltySvc.SetPenaltyRate(context.Background(), b.Now(), amount)
	if err != nil {
		return err
	}
	return c.Reply(fmt.Sprintf("💸 Ставка штрафа на неделю %s — %s установлена: %d сом за пропуск.",
		week.StartDate.Format("02.01.2006"), week.EndDate.Format("02.01.2006"), amount))
}

func (b *Bot) handleMarkPaid(c telebot.Context) error {
	username := strings.TrimPrefix(strings.TrimSpace(c.Data()), "@")
	if username == "" {
		return c.Reply("Использование: /mark_paid @username")
	}

	ctx := context.Background()
	week, err := b.penaltySvc.GetLastClosedWeek(ctx)
	if err != nil {
		return err
	}
	if week == nil {
		return c.Reply("Ещё нет ни одной закрытой недели со штрафами.")
	}

	user, err := b.penaltySvc.FindUserByUsername(ctx, username)
	if err != nil {
		return c.Reply(fmt.Sprintf("Пользователь @%s не найден.", username))
	}

	if err := b.penaltySvc.MarkPaid(ctx, week.ID, user.ID); err != nil {
		return err
	}
	return c.Reply(fmt.Sprintf("✅ Отмечено: @%s оплатил штраф за неделю %s — %s.",
		username, week.StartDate.Format("02.01.2006"), week.EndDate.Format("02.01.2006")))
}

// handleForgive waives a user's penalty for the last closed week (e.g. they
// had a good excuse) — it zeroes the amount owed and removes it from the
// fund, as opposed to /mark_paid which just records that money changed hands.
func (b *Bot) handleForgive(c telebot.Context) error {
	username := strings.TrimPrefix(strings.TrimSpace(c.Data()), "@")
	if username == "" {
		return c.Reply("Использование: /forgive @username")
	}

	ctx := context.Background()
	week, err := b.penaltySvc.GetLastClosedWeek(ctx)
	if err != nil {
		return err
	}
	if week == nil {
		return c.Reply("Ещё нет ни одной закрытой недели со штрафами.")
	}

	user, err := b.penaltySvc.FindUserByUsername(ctx, username)
	if err != nil {
		return c.Reply(fmt.Sprintf("Пользователь @%s не найден.", username))
	}

	amount, err := b.penaltySvc.Forgive(ctx, week.ID, user.ID)
	if errors.Is(err, service.ErrNoPenalty) {
		return c.Reply(fmt.Sprintf("У @%s нет штрафа за неделю %s — %s.",
			username, week.StartDate.Format("02.01.2006"), week.EndDate.Format("02.01.2006")))
	}
	if err != nil {
		return err
	}
	if amount == 0 {
		return c.Reply(fmt.Sprintf("У @%s штраф за эту неделю и так 0 сом — прощать нечего.", username))
	}

	return c.Reply(fmt.Sprintf("🤝 Штраф %d сом для @%s за неделю %s — %s прощён.",
		amount, username, week.StartDate.Format("02.01.2006"), week.EndDate.Format("02.01.2006")))
}

// handleSetAdmin grants a registered user admin rights at runtime (stored in
// users.is_admin), without touching config.json. The target must have
// already talked to the bot at least once (/start) to exist in the DB.
func (b *Bot) handleSetAdmin(c telebot.Context) error {
	username := strings.TrimPrefix(strings.TrimSpace(c.Data()), "@")
	if username == "" {
		return c.Reply("Использование: /setadmin @username")
	}

	ctx := context.Background()
	user, err := b.svc.FindUserByUsername(ctx, username)
	if err != nil {
		return c.Reply(fmt.Sprintf("Пользователь @%s не найден — он должен сначала написать боту /start в личке.", username))
	}

	if err := b.svc.SetAdmin(ctx, user.TelegramID, true); err != nil {
		return err
	}
	return c.Reply(fmt.Sprintf("✅ @%s назначен администратором.", username))
}

// handleUnsetAdmin revokes runtime-granted admin rights. Admins listed in
// config.json (admin_telegram_ids) can't be revoked this way — that list is
// the permanent bootstrap set and only changes by editing the config.
func (b *Bot) handleUnsetAdmin(c telebot.Context) error {
	username := strings.TrimPrefix(strings.TrimSpace(c.Data()), "@")
	if username == "" {
		return c.Reply("Использование: /unsetadmin @username")
	}

	ctx := context.Background()
	user, err := b.svc.FindUserByUsername(ctx, username)
	if err != nil {
		return c.Reply(fmt.Sprintf("Пользователь @%s не найден.", username))
	}

	if b.cfg.IsAdmin(user.TelegramID) {
		return c.Reply(fmt.Sprintf("@%s задан супер администратором — снять права невозможно.", username))
	}

	if err := b.svc.SetAdmin(ctx, user.TelegramID, false); err != nil {
		return err
	}
	return c.Reply(fmt.Sprintf("✅ @%s больше не администратор.", username))
}
