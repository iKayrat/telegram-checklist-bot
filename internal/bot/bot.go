package bot

import (
	"context"
	"fmt"
	"time"

	telebot "gopkg.in/telebot.v3"

	"github.com/ikkairat/telegram-checklist-bot/internal/config"
	"github.com/ikkairat/telegram-checklist-bot/internal/service"
)

// Bot wires a telebot instance to the checklist and penalty services. It
// knows nothing about SQL — all persistence goes through those services.
type Bot struct {
	*telebot.Bot
	cfg        *config.Config
	svc        *service.ChecklistService
	penaltySvc *service.PenaltyService
	loc        *time.Location
}

func New(cfg *config.Config, svc *service.ChecklistService, penaltySvc *service.PenaltyService) (*Bot, error) {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", cfg.Timezone, err)
	}

	tb, err := telebot.NewBot(telebot.Settings{
		Token:  cfg.BotToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, fmt.Errorf("init telebot: %w", err)
	}

	b := &Bot{Bot: tb, cfg: cfg, svc: svc, penaltySvc: penaltySvc, loc: loc}
	b.registerUserHandlers()
	b.registerAdminHandlers()
	return b, nil
}

// Now returns the current time in the bot's configured timezone.
func (b *Bot) Now() time.Time {
	return time.Now().In(b.loc)
}

// adminOnly is a telebot middleware that rejects non-admin senders. A sender
// counts as admin if they're listed in config.json (admin_telegram_ids —
// the permanent, bootstrap admins) or were granted admin rights at runtime
// via /setadmin.
func (b *Bot) adminOnly(next telebot.HandlerFunc) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		sender := c.Sender()
		if sender == nil {
			return c.Reply("⛔ Команда доступна только администратору.")
		}
		if b.cfg.IsAdmin(sender.ID) {
			return next(c)
		}

		isAdmin, err := b.svc.IsAdmin(context.Background(), sender.ID)
		if err != nil {
			return err
		}
		if !isAdmin {
			return c.Reply("⛔ Команда доступна только администратору.")
		}
		return next(c)
	}
}
