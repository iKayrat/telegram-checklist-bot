# Деплой на Raspberry Pi (без Docker)

## 1. Сборка (кросс-компиляция с обычного компьютера)

Определите архитектуру своего Pi:
- Pi 3/4/5 с 64-битной Raspberry Pi OS — `arm64`
- Pi Zero/1/2 или 32-битная Raspberry Pi OS — `arm` (плюс `GOARM`: `6` для Zero/1, `7` для 2/3/4)

```sh
# 64-бит (Pi 3/4/5, 64-bit OS)
GOOS=linux GOARCH=arm64 go build -o checklist-bot ./cmd/bot

# 32-бит (Pi Zero/1/2, 32-bit OS)
GOOS=linux GOARCH=arm GOARM=6 go build -o checklist-bot ./cmd/bot
```

`modernc.org/sqlite` — чистый Go без cgo, так что `CGO_ENABLED=0` (по умолчанию
при кросс-компиляции) даёт статический бинарник, который просто копируется на Pi.

## 2. Файлы на Pi

Создайте `/home/pi/checklist-bot/` и положите туда:

```
checklist-bot/
├── checklist-bot        # собранный бинарник
├── config.json          # скопировать из config.example.json и заполнить
├── migrations/           # содержимое migrations/ из репозитория
├── backup.sh             # deploy/backup.sh
└── data/                 # создастся автоматически (db_path, reports_dir)
```

```sh
scp checklist-bot config.json backup.sh pi@<pi-host>:/home/pi/checklist-bot/
scp -r migrations pi@<pi-host>:/home/pi/checklist-bot/
ssh pi@<pi-host> chmod +x /home/pi/checklist-bot/checklist-bot /home/pi/checklist-bot/backup.sh
```

## 3. systemd-сервис

```sh
sudo cp deploy/checklist-bot.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now checklist-bot
sudo journalctl -u checklist-bot -f   # логи (slog в stdout → journal)
```

Если пользователь или пути отличаются от `pi` / `/home/pi/checklist-bot`,
поправьте `User=` и пути в `checklist-bot.service` перед копированием.

## 4. Ежедневный бэкап SQLite

В файле крутятся реальные деньги (штрафы), поэтому бэкап обязателен (см. §7 архитектуры):

```sh
crontab -e
# добавить строку:
0 3 * * * /home/pi/checklist-bot/backup.sh
```

## 5. Обновление бинарника

```sh
sudo systemctl stop checklist-bot
scp checklist-bot pi@<pi-host>:/home/pi/checklist-bot/
sudo systemctl start checklist-bot
```

Миграции применяются автоматически при старте (`storage.Open`), так что
новые файлы в `migrations/` достаточно скопировать перед перезапуском.
