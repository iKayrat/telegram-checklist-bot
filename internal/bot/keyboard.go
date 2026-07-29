package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	telebot "gopkg.in/telebot.v3"

	"github.com/ikkairat/telegram-checklist-bot/internal/domain"
	"github.com/ikkairat/telegram-checklist-bot/internal/service"
)

// checkinBtnUnique is the callback unique id for checklist task buttons;
// the task id and the poll's date travel as the button's Data payload
// (joined with "|", split back out via telebot's Context.Args()) — so a
// press on a stale (yesterday's or earlier) message can be told apart from
// one on today's actual poll instead of always being treated as "today".
const checkinBtnUnique = "chk"

// BuildChecklistKeyboard renders one button per active task, tagged with
// pollDate. Any group member can press any button — the handler resolves
// the task from the button and the user from the callback sender, so each
// press only affects the presser's own row in the message text.
func BuildChecklistKeyboard(tasks []domain.Task, pollDate time.Time) *telebot.ReplyMarkup {
	dateStr := pollDate.Format("2006-01-02")
	markup := &telebot.ReplyMarkup{}
	rows := make([]telebot.Row, 0, len(tasks))
	for i, t := range tasks {
		btn := markup.Data(fmt.Sprintf("%d. %s", i+1, t.Title), checkinBtnUnique,
			strconv.FormatInt(t.ID, 10), dateStr)
		rows = append(rows, markup.Row(btn))
	}
	markup.Inline(rows...)
	return markup
}

// BuildChecklistText renders the checklist message: the task legend and a
// per-user completion line, e.g. "Иванов: 1✅ 2❌".
func BuildChecklistText(date time.Time, tasks []domain.Task, statuses []service.UserStatus) string {
	var b strings.Builder

	fmt.Fprintf(&b, "📋 Чек-лист на %s\nОтмечай свои пункты 👇\n\n", date.Format("02.01.2006"))

	b.WriteString("Задачи:\n")
	for i, t := range tasks {
		fmt.Fprintf(&b, "%d. %s\n", i+1, t.Title)
	}

	b.WriteString("\nСтатус:\n")
	for _, st := range statuses {
		b.WriteString(st.User.FullName)
		b.WriteString(": ")
		for i, t := range tasks {
			mark := "❌"
			if st.Done[t.ID] {
				mark = "✅"
			}
			fmt.Fprintf(&b, "%d%s ", i+1, mark)
		}
		b.WriteString("\n")
	}

	return b.String()
}
