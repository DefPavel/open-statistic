# OpenVPN Traffic Statistics

Backend на Go + SQLite для анализа трафика OpenVPN: пользователи, объём трафика, текущие подключения.

## Требования

- Go 1.21+
- OpenVPN с настроенным status-файлом

Добавьте в конфиг OpenVPN сервера:
```
status /var/log/openvpn/status.log
status-version 2
```

## Установка

```bash
go mod download
go build -o openstat ./cmd/server
```

## Запуск

```bash
./openstat [флаги]
```

Переменные окружения (или флаги):
| Env | Флаг | По умолчанию |
|-----|------|--------------|
| `PORT` | `-addr` | `8080` |
| `API_KEY` | — | пусто (без аутентификации) |
| `DB_PATH` | `-db` | `./openstat.db` |
| `STATUS_PATH` | `-status` | `/var/log/openvpn/status.log` |
| `INTERVAL` | `-interval` | `60s` |
| `RETENTION` | `-retention` | `1000` |
| `ALLOWED_PATHS` | — | `/var/log/openvpn` (для /collect) |

Флаги переопределяют env.

## API

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/health` | Проверка состояния |
| GET | `/stats` | Сводка: подключения, пользователи, трафик (сессии и накопленный) |
| GET | `/users` | Список пользователей |
| GET | `/users/:name/traffic` | Трафик в текущей сессии |
| GET | `/users/:name/total` | Накопленный трафик за всё время |
| GET | `/traffic` | Трафик в сессиях всех пользователей |
| GET | `/traffic/total` | Накопленный трафик всех пользователей |
| GET | `/connected` | Текущие подключения |
| POST | `/collect?path=...` | Принудительный сбор из указанного status-файла |

Для трафика и stats добавьте `?human=1` — ответ в MB/GB.

## Примеры

```bash
# Список пользователей
curl http://localhost:8080/users

# С API-ключом (если задан API_KEY)
curl -H "X-API-Key: your-secret-key" http://localhost:8080/users
# или
curl -H "Authorization: Bearer your-secret-key" http://localhost:8080/users

# Трафик пользователя user1
curl http://localhost:8080/users/user1/traffic

# Текущие подключения
curl http://localhost:8080/connected

# Принудительный сбор (path должен быть внутри ALLOWED_PATHS)
curl -X POST "http://localhost:8080/collect?path=/var/log/openvpn/status.log"
```

## Безопасность (Production)

1. **API_KEY** — обязательно задайте в production. Иначе данные о пользователях и трафике доступны всем.
2. **Сеть** — не выносите API в интернет. Используйте внутреннюю сеть, VPN или reverse proxy с аутентификацией.
3. **HTTPS** — ставьте за nginx/traefik с TLS. Приложение отдаёт только HTTP.
4. **Path traversal** — `/collect?path=` проверяется на `ALLOWED_PATHS`. Не используйте широкие директории.
5. **Брандмауэр** — ограничьте доступ по IP (только доверенные хосты).
6. **Обновления** — регулярно обновляйте образ и зависимости.

## Тестовый прогон

```bash
./openstat -status=./config/openvpn-status.sample -interval=5s
```

## Docker

```bash
docker compose up -d
```

API: http://localhost:8080

Конфигурация через env в `docker-compose.yml` или `.env` в корне проекта. По умолчанию — sample status-файл из `data/openvpn-status/status.log`. Для работы с реальным OpenVPN измените volume:

```yaml
volumes:
  - /var/log/openvpn:/var/log/openvpn:ro  # путь на хосте с OpenVPN
```
