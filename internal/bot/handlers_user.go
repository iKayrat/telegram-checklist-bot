package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	telebot "gopkg.in/telebot.v3"
)

func fullNameOf(u *telebot.User) string {
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		name = u.Username
	}
	return name
}

func (b *Bot) registerUserHandlers() {
	b.Handle("/start", b.handleStart)
	b.Handle("/report", b.handleReport)
	b.Handle("/fund", b.handleFund)
	b.Handle(&telebot.Btn{Unique: checkinBtnUnique}, b.handleCheckinToggle)
}

// handleStart registers the sender so the bot can DM them reminders later
// (see architecture doc §7: the bot can only message users who opened a
// private chat with it first). Only handled in private chats.
func (b *Bot) handleStart(c telebot.Context) error {
	if c.Chat().Type != telebot.ChatPrivate {
		return nil
	}

	sender := c.Sender()
	if _, err := b.svc.RegisterUser(context.Background(), sender.ID, sender.Username, fullNameOf(sender)); err != nil {
		return err
	}
	return c.Send("Салам! Ты зарегистрирован и теперь будешь получать личные напоминания о чек-листе.")
}

// handleCheckinToggle handles a press on any checklist task button. It
// toggles the pressing user's completion state for that task on today's
// date and re-renders the shared checklist message.
func (b *Bot) handleCheckinToggle(c telebot.Context) error {
	taskID, err := strconv.ParseInt(c.Data(), 10, 64)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "Некорректная задача"})
	}

	today := b.Now()
	checked, err := b.svc.ToggleCheckin(context.Background(), c.Sender().ID, taskID, today)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{
			Text:      "Сначала напиши мне /start в личных сообщениях",
			ShowAlert: true,
		})
	}

	answer := "☑️ Снято"
	if checked {
		answer = "✅ Отмечено"
	}
	if err := c.Respond(&telebot.CallbackResponse{Text: answer}); err != nil {
		return err
	}

	tasks, err := b.svc.ActiveTasks(context.Background())
	if err != nil {
		return err
	}
	statuses, err := b.svc.BuildStatuses(context.Background(), today)
	if err != nil {
		return err
	}
	return c.Edit(BuildChecklistText(today, tasks, statuses), BuildChecklistKeyboard(tasks))
}

// handleReport replies (in DM) with the sender's own progress in the
// current week: how many task-slots they've completed vs. missed so far.
func (b *Bot) handleReport(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	user, err := b.svc.RegisterUser(ctx, sender.ID, sender.Username, fullNameOf(sender))
	if err != nil {
		return err
	}

	start, end, total, completed, missed, err := b.penaltySvc.UserWeekStatus(ctx, b.Now(), user.ID)
	if err != nil {
		return err
	}

	return c.Reply(fmt.Sprintf(
		"📊 Твоя статистика за неделю %s — %s:\nВыполнено: %d/%d\nПропущено: %d",
		start.Format("02.01.2006"), end.Format("02.01.2006"), completed, total, missed))
}

// handleFund replies with the current total of the penalty fund.
func (b *Bot) handleFund(c telebot.Context) error {
	total, err := b.penaltySvc.FundTotal(context.Background())
	if err != nil {
		return err
	}
	return c.Reply(fmt.Sprintf("🏦 Текущий фонд штрафов: %d сом", total))
}
