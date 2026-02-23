# OpenVPN Traffic Statistics

Go + SQLite — анализ трафика OpenVPN: пользователи, трафик, подключения.

## Требования

OpenVPN с `status /var/log/openvpn/status.log` в конфиге.

## Быстрый старт

```bash
# Docker
cp .env.example .env
docker compose up -d
```

API: `http://localhost:8080`

## Запуск локально

```bash
go build -o openstat ./cmd/server
./openstat -status=./config/openvpn-status.sample
```

## API

| Путь | Описание |
|------|----------|
| `GET /health` | Проверка |
| `GET /stats` | Сводка |
| `GET /users` | Пользователи |
| `GET /users/:name/traffic` | Трафик в сессии |
| `GET /users/:name/total` | Накопленный трафик |
| `GET /traffic` | Трафик всех |
| `GET /traffic/total` | Накопленный всех |
| `GET /connected` | Подключённые |

`?human=1` — вывод в MB/GB. С `API_KEY`: заголовок `X-API-Key` или `Authorization: Bearer <key>`.

## Конфиг

| Env | По умолчанию |
|-----|--------------|
| `PORT` | `8080` |
| `API_KEY` | пусто |
| `DB_PATH` | `./openstat.db` |
| `STATUS_PATH` | `/var/log/openvpn/status.log` |
| `OPENVPN_STATUS_DIR` | `./data/openvpn-status` |

## Production

→ [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) — обязательно `API_KEY`, HTTPS, бэкап.
