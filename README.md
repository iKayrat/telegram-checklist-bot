# Telegram Checklist + Penalty Bot

Telegram-бот для группы друзей (~15 человек, Бишкек): каждый день в заданное
время бот присылает в группу чек-лист задач с inline-кнопками, за час до
конца дня напоминает тем, кто не отметился, а раз в неделю считает штрафы за
пропуски, формирует PDF-отчёт и публикует его в группе и в личке админа.

Полное описание архитектуры и логики штрафов — в [`bot-architecture.md`](bot-architecture.md).

## Стек

- Go 1.22+ ([`gopkg.in/telebot.v3`](https://pkg.go.dev/gopkg.in/telebot.v3) — Telegram API)
- SQLite (`modernc.org/sqlite`, без cgo) + `sqlx` + `golang-migrate/migrate`
- `robfig/cron/v3` — планировщик (ежедневный чек-лист, напоминания, недельный отчёт)
- `johnfercher/maroto/v2` — генерация PDF-отчёта

## Быстрый старт

```sh
cp config.example.json config.json   # заполнить bot_token, group_chat_id, admin_telegram_ids
make run
```

## Структура проекта

```
cmd/bot            — точка входа, связывает конфиг/БД/бота/планировщик
internal/domain     — структуры данных (User, Task, Checkin, Week, Penalty...)
internal/storage     — репозитории поверх sqlx + миграции
internal/service     — бизнес-логика (чек-лист, штрафы)
internal/bot          — telebot: хендлеры команд и чек-листа
internal/scheduler    — cron-задачи
internal/pdf           — генерация недельного PDF-отчёта
migrations/            — SQL-миграции (golang-migrate)
deploy/                — systemd unit + инструкция по деплою на Raspberry Pi
```

## Makefile

| Команда | Что делает |
|---|---|
| `make run` | запуск локально (`go run ./cmd/bot`) |
| `make build` | сборка бинарника под текущую ОС |
| `make build-arm64` / `make build-arm` | кросс-компиляция под Raspberry Pi |
| `make test` | `go test ./...` |
| `make vet` | `go vet ./...` |
| `make tidy` | `go mod tidy` |

## Команды бота

### Админ

| Команда | Описание |
|---|---|
| `/add <название>` | добавить задачу в чек-лист |
| `/remove <id>` | убрать задачу из чек-листа (id — см. `/list`) |
| `/list` | таблица активных задач с их id |
| `/post_checklist` | отправить сегодняшний чек-лист в группу вручную (обычно это делает cron по `daily_poll_time`) |
| `/post_report` | вручную закрыть текущую неделю и разослать PDF-отчёт (обычно это делает cron по `weekly_report_day`/`weekly_report_time`). ⚠️ Это не предпросмотр — неделя закрывается по-настоящему, как и при плановом запуске |
| `/setpenalty <сумма>` | задать ставку штрафа (сом) за один пропуск на текущую неделю |
| `/mark_paid @username` | отметить, что участник оплатил штраф за последнюю закрытую неделю |
| `/forgive @username` | простить (обнулить) штраф участнику за последнюю закрытую неделю |
| `/setadmin @username` | назначить участника администратором (нужно, чтобы он уже написал `/start`) |
| `/unsetadmin @username` | снять права администратора (не действует на админов из `config.json`) |

### Участники

| Команда | Описание |
|---|---|
| `/start` | зарегистрироваться в личке — без этого бот не сможет присылать личные напоминания и принимать отметки в чек-листе |
| `/report` | своя статистика выполнения за текущую неделю (выполнено/пропущено) |
| `/fund` | текущий размер общего фонда штрафов |

## Деплой

Инструкция по сборке под ARM и запуску на Raspberry Pi (systemd, без Docker,
бэкапы SQLite) — в [`deploy/README.md`](deploy/README.md).
