-- участники
CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_id   INTEGER UNIQUE NOT NULL,
    username      TEXT,
    full_name     TEXT NOT NULL,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    is_active     INTEGER NOT NULL DEFAULT 1,
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
    message_id  INTEGER NOT NULL,
    is_closed   INTEGER NOT NULL DEFAULT 0,
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
    penalty_amount INTEGER NOT NULL,
    is_closed      INTEGER NOT NULL DEFAULT 0,
    report_pdf_path TEXT
);

-- итоговые штрафы по каждому участнику за неделю
CREATE TABLE penalties (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    week_id       INTEGER NOT NULL REFERENCES weeks(id),
    user_id       INTEGER NOT NULL REFERENCES users(id),
    total_tasks   INTEGER NOT NULL,
    missed_tasks  INTEGER NOT NULL,
    amount        INTEGER NOT NULL,
    is_paid       INTEGER NOT NULL DEFAULT 0,
    UNIQUE(week_id, user_id)
);

-- общий накопительный фонд
CREATE TABLE fund_ledger (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    week_id     INTEGER NOT NULL REFERENCES weeks(id),
    amount      INTEGER NOT NULL,
    note        TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
