# Архитектура Telegram-бота "Чек-лист + штрафы" (Go)

## 1. Суть системы

Группа из ~15 друзей в Бишкеке. Каждый день в фиксированное время бот присылает
в группу сообщение со списком задач (например: "прочитать книгу", "спорт" и т.д.)
с inline-кнопками. Каждый участник отмечает свои выполненные пункты. За час до
конца дня — напоминание тем, кто не отметился полностью. В конце дня то, что не
отмечено — засчитывается как "нет". Раз в неделю бот считает штрафы (фикс. сумма
за каждый пропуск, например 1000 сом за неделю по логике, которую вы зададите),
формирует PDF-отчёт и публикует в группу + отправляет админу.

---

## 2. Технологический стек

| Компонент | Выбор | Почему |
|---|---|---|
| Язык | Go 1.22+ | как и просили |
| Telegram API | `gopkg.in/telebot.v3` | чище API, встроенные middleware, удобная работа с inline-клавиатурами и группами, активно поддерживается |
| Планировщик задач | `github.com/robfig/cron/v3` | cron-выражения с таймзоной, идеально для "каждый день в 21:00 Asia/Bishkek" и "раз в неделю по воскресеньям" |
| БД | **SQLite** (`modernc.org/sqlite`, чистый Go, без cgo) | на 15 человек и низкую нагрузку PostgreSQL избыточен; SQLite = один файл, простой бэкап, простой деплой. Если позже вырастет — миграция на Postgres тем же слоем репозиториев не проблема |
| Работа с БД | `sqlx` + обычные SQL-миграции (`golang-migrate/migrate`) | без магии ORM, полный контроль над запросами, легко читать |
| PDF-отчёты | `github.com/johnfercher/maroto/v2` | таблицы, стили, готовые компоненты — быстрее собрать презентабельный отчёт, чем на голом `gofpdf` |
| Конфиг | `config.json` + стандартный `encoding/json` (или `json.Unmarshal` в структуру `Config`) | просто, читаемо, легко редактировать вручную на Pi без пересборки бинарника |
| Логирование | `log/slog` (стандартная библиотека) | ничего доп. не нужно, structured logging из коробки |
| Таймзона | `time.LoadLocation("Asia/Bishkek")` захардкожена | все участники в одном поясе, как вы сказали |
| Деплой | Raspberry Pi, systemd-сервис, без Docker | бинарник Go компилируется под ARM (`GOARCH=arm64` или `arm` в зависимости от модели Pi), т.к. `modernc.org/sqlite` — чистый Go без cgo, кросс-компиляция с обычного компьютера тривиальна (`GOOS=linux GOARCH=arm64 go build`) |

---

## 3. Структура проекта (стандартный Go layout)

```
telegram-checklist-bot/
├── cmd/
│   └── bot/
│       └── main.go                 # точка входа: конфиг, БД, бот, cron — всё связывается тут
├── internal/
│   ├── config/
│   │   └── config.go                # структура Config + чтение config.json
│   ├── domain/
│   │   ├── user.go                  # структуры User, Task, Checkin, Week, Penalty
│   │   ├── task.go
│   │   ├── checkin.go
│   │   └── penalty.go
│   ├── storage/
│   │   ├── sqlite.go                 # подключение, миграции
│   │   ├── user_repo.go
│   │   ├── task_repo.go
│   │   ├── checkin_repo.go
│   │   └── penalty_repo.go
│   ├── service/
│   │   ├── checklist_service.go     # логика: создать дневной опрос, зафиксировать день
│   │   ├── penalty_service.go       # расчёт штрафов за неделю, фонд
│   │   └── report_service.go        # сборка данных для PDF
│   ├── bot/
│   │   ├── bot.go                    # инициализация telebot, middleware (проверка админа и т.п.)
│   │   ├── handlers_user.go          # обработка нажатий на чек-бокс
│   │   ├── handlers_admin.go         # /addtask, /removetask, /settime, /setpenalty, /forgive
│   │   └── keyboard.go               # генерация inline-клавиатуры чек-листа
│   ├── scheduler/
│   │   └── scheduler.go              # cron-задачи: отправка чек-листа, напоминание, закрытие дня, недельный отчёт
│   └── pdf/
│       └── weekly_report.go          # генерация PDF через maroto
├── migrations/
│   ├── 0001_init.sql
│   └── ...
├── deploy/
│   └── checklist-bot.service   # systemd unit-файл для автозапуска на Pi
├── go.mod
└── config.example.json         # пример конфига (копируется в config.json на Pi)
```

Принцип: **bot** ничего не знает о SQL, **storage** ничего не знает о Telegram,
**service** — связующий слой с бизнес-логикой (штрафы, проценты, дедлайны).
Это позволяет тестировать расчёт штрафов без поднятия бота.

---

## 4.1. Пример config.json

```json
{
  "bot_token": "123456:ABC-your-telegram-bot-token",
  "group_chat_id": -1001234567890,
  "admin_telegram_ids": [111111111, 222222222],
  "timezone": "Asia/Bishkek",
  "daily_poll_time": "21:00",
  "reminder_before_deadline_minutes": 60,
  "day_deadline_time": "23:59",
  "weekly_report_day": "sunday",
  "weekly_report_time": "23:00",
  "db_path": "./data/bot.db",
  "reports_dir": "./data/reports"
}
```

## 4.2. Схема БД (SQLite)

```sql
-- участники
CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_id   INTEGER UNIQUE NOT NULL,
    username      TEXT,
    full_name     TEXT NOT NULL,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    is_active     INTEGER NOT NULL DEFAULT 1,  -- на случай если кто-то выходит из группы
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- задачи чек-листа (общие для всех, редактируются админом)
CREATE TABLE tasks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT NOT NULL,
    is_active   INTEGER NOT NULL DEFAULT 1,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ежедневная рассылка чек-листа (одно сообщение в группе = один "лист")
CREATE TABLE daily_polls (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    poll_date   DATE NOT NULL UNIQUE,
    message_id  INTEGER NOT NULL,          -- id сообщения в Telegram, чтобы редактировать клавиатуру
    is_closed   INTEGER NOT NULL DEFAULT 0,  -- закрыт ли день (после дедлайна)
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- отметки: кто что выполнил в какой день
CREATE TABLE checkins (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    task_id     INTEGER NOT NULL REFERENCES tasks(id),
    poll_date   DATE NOT NULL,
    checked     INTEGER NOT NULL DEFAULT 0,
    checked_at  DATETIME,
    UNIQUE(user_id, task_id, poll_date)
);

-- недели (для агрегации штрафов)
CREATE TABLE weeks (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    start_date     DATE NOT NULL,
    end_date       DATE NOT NULL,
    penalty_amount INTEGER NOT NULL,   -- сумма штрафа за неделю в сомах (за пропуск/за %, см. п.5)
    is_closed      INTEGER NOT NULL DEFAULT 0,
    report_pdf_path TEXT
);

-- итоговые штрафы по каждому участнику за неделю
CREATE TABLE penalties (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    week_id       INTEGER NOT NULL REFERENCES weeks(id),
    user_id       INTEGER NOT NULL REFERENCES users(id),
    total_tasks   INTEGER NOT NULL,   -- сколько всего пунктов должен был выполнить за неделю
    missed_tasks  INTEGER NOT NULL,   -- сколько пропустил
    amount        INTEGER NOT NULL,   -- сумма к оплате в сомах
    is_paid        INTEGER NOT NULL DEFAULT 0,  -- админ отмечает вручную, что оплатил
    UNIQUE(week_id, user_id)
);

-- общий накопительный фонд (можно и просто SUM(amount) по penalties, но отдельная
-- таблица удобна если фонд не равен сумме штрафов 1:1, напр. частичные оплаты)
CREATE TABLE fund_ledger (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    week_id     INTEGER NOT NULL REFERENCES weeks(id),
    amount      INTEGER NOT NULL,
    note        TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

---

## 5. Логика начисления штрафа (зафиксировано — Вариант А)

`штраф = (кол-во пропущенных отметок за неделю) × (ставка за 1 пропуск)`

Например: ставка 100 сом/пропуск, участник пропустил 5 отметок за неделю → 500 сом.
Ставка задаётся админом командой `/setpenalty <сумма>` и хранится в `weeks.penalty_amount`
как "цена одного пропуска" для этой недели (так можно менять ставку от недели к неделе,
не трогая код).

Расчёт в `penalty_service.go`:
```go
missed := totalTasks - completedTasks
amount := missed * penaltyPerMiss
```

---

## 6. Сценарий работы бота по шагам

1. **Настройка (админ, разово):**
   `/addtask Прочитать книгу`, `/addtask Спорт 30 мин` и т.д.
   `/settime 21:00` — время ежедневной отправки.
   `/setpenalty 100` — ставка/порог по выбранной формуле.

2. **Каждый день в заданное время (cron):**
   Бот отправляет в группу сообщение вида:
   ```
   📋 Чек-лист на 24.07.2026
   Отмечай свои пункты 👇
   ```
   с inline-клавиатурой, где на каждого пользователя генерируется **персональная
   строка кнопок** (Telegram позволяет любому нажимать любую кнопку в общем
   сообщении, поэтому бот проверяет `callback.Sender.ID` и обновляет только
   тот checkbox, который относится к нажавшему). Практическая реализация:
   при нажатии бот отвечает `callback answer` "✅ отмечено" лично нажавшему,
   а текст сообщения обновляется построчно (Иванов: 📖✅ 🏃❌ | Петров: 📖❌ 🏃❌...).

3. **За час до конца дня:** cron проверяет, у кого остались непроставленные
   пункты, и шлёт **личное** напоминание в ЛС (для этого у бота должен быть
   стартован диалог с каждым участником — нужно, чтобы все один раз нажали
   `/start` боту в личке).

4. **В момент дедлайна:** cron закрывает `daily_polls.is_closed = 1`,
   все непроставленные `checkins` на этот день фиксируются как `checked = 0`
   (или просто отсутствие записи трактуется как "нет" при подсчёте — так даже
   проще, лишняя запись не нужна).

5. **Раз в неделю (например, воскресенье 23:00):**
   - Считается `penalties` по каждому участнику согласно выбранной формуле.
   - Генерируется PDF через `maroto`: таблица участник × дни недели × ✅/❌,
     итоговый % выполнения, сумма штрафа, и сводка по фонду.
   - PDF публикуется в группу + отправляется админу в ЛС.
   - Новая запись в `weeks` создаётся на следующую неделю.

6. **Команды для просмотра:**
   - `/report` — участник в личке получает свою мини-статистику текстом.
   - `/fund` — админ (или все) видят текущий размер фонда.
   - `/mark_paid @user` — админ отмечает, что человек оплатил штраф.

---

## 7. Технические нюансы, которые важно учесть заранее

- **Идентификация пользователей в группе**: бот должен один раз собрать `telegram_id`
  всех 15 участников (через `/start` в личке или через отслеживание сообщений в группе)
  до того, как начнёт слать личные напоминания — иначе не сможет написать в ЛС тем,
  кто не начинал с ним диалог.
- **Часовой пояс**: у `cron.New(cron.WithLocation(loc))` обязательно указать
  `Asia/Bishkek`, иначе сервер (если хостится не в Бишкеке) будет слать не вовремя.
- **Персистентность**: SQLite-файл и папка с PDF-отчётами должны быть в Docker volume,
  иначе при пересборке контейнера всё потеряется.
- **Редактирование сообщения с чек-листом**: Telegram ограничивает частоту
  редактирования одного сообщения — при 15 участниках, часто нажимающих кнопки,
  стоит делать debounce (например, редактировать не чаще раза в 2–3 секунды),
  чтобы не словить rate limit.
- **Бэкап**: раз в день делать копию sqlite-файла (простой cron + `cp`), учитывая,
  что там крутятся реальные деньги (штрафы) — потеря данных болезненна.

---

## 8. Итоговые решения (зафиксировано)

1. Формула штрафа — **Вариант А**: сумма = кол-во пропусков × ставка за пропуск.
2. Список задач — **фиксированный**, одинаковый каждый день (без привязки к дню недели).
   Меняется только вручную через `/addtask` / `/removetask`, если понадобится в будущем.
3. Хостинг — **Raspberry Pi**, без Docker, автозапуск через systemd.
4. Штраф — **просто учёт в общий фонд** (`fund_ledger`), без привязки "кто кому",
   без платёжной интеграции. Админ вручную отмечает `/mark_paid @user`, когда получил деньги.
5. Интеграция с реальными переводами — **не нужна** на этом этапе.

Раз задачи фиксированы, схему можно чуть упростить: таблица `tasks` не нуждается
в поле привязки к дню недели — она уже спроектирована под этот случай (п.4 схемы БД
не меняется).

## 9. Что дальше

Стек и архитектура полностью зафиксированы. Следующий шаг — начать код в таком порядке
(логично реализовывать и тестировать по частям):

1. `internal/domain` — структуры данных.
2. `migrations/0001_init.sql` — создание таблиц из раздела 4.
3. `internal/storage` — репозитории (CRUD поверх sqlx).
4. `internal/bot` — регистрация бота, команды админа (`/addtask` и т.д.), затем обработка
   чек-листа.
5. `internal/scheduler` — cron-задачи (отправка листа, напоминание, закрытие дня, недельный расчёт).
6. `internal/pdf` — генерация отчёта, когда расчёт штрафов уже работает и есть тестовые данные.
7. `deploy/checklist-bot.service` + инструкция по сборке под ARM и запуску на Pi.

Могу начать писать код по этому плану — например, с `internal/domain` + миграций,
либо сразу с ядра (обработка нажатий на чек-лист), если хотите сначала увидеть
рабочий MVP в группе. Как удобнее двигаться?
