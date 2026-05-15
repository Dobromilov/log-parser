# log-parser

Микросервис на Go для разбора архивов с диагностическими логами InfiniBand. Сервис читает архив из локальной папки `data/`, парсит секции `db_csv`, сохраняет результат в PostgreSQL и отдает данные по REST API.

Проект сделан под тестовое задание: минимальный HTTP-слой на `net/http`, PostgreSQL через Docker Compose, автоматические SQL-миграции при старте приложения.

## Что делает сервис

1. Принимает путь до zip-архива через `POST /api/v1/parse/`.
2. Ищет этот архив внутри папки `data/`.
3. Разбирает секции:
   - `NODES`
   - `PORTS`
   - `SYSTEM_GENERAL_INFORMATION`
4. Собирает сущности:
   - лог загрузки;
   - узлы fabric: host/switch;
   - порты узлов;
   - дополнительную информацию об узлах.
5. Валидирует связи: порт или node info не могут ссылаться на неизвестный node.
6. Сохраняет данные в PostgreSQL.
7. Отдает метаданные лога, топологию, детали узла и порты узла через API.

Если архив битый, не найден, содержит некорректные секции или нарушает связи между сущностями, запрос отклоняется с ошибкой.

## Топология

Сейчас топология строится из `nodes` и `ports`:

- `nodes` превращаются в узлы графа;
- поле `type` делит узлы на группы `host`, `switch`, `unknown`;
- порты привязаны к конкретным узлам через FK `ports.node_id`;
- в ответе `/api/v1/topology/{log_id}` возвращаются узлы, группы и поле `links`.

Поле `links` пока возвращается пустым массивом, потому что в текущих секциях `NODES` и `PORTS` нет надежной информации о второй стороне соединения. В production-версии связи можно строить, если в логах есть таблицы/секции с adjacency-информацией: например cable/link map, remote port GUID, remote node GUID, LID routing или direct route dump. Тогда можно добавить таблицу `links` и хранить пары `source_node/source_port -> target_node/target_port` с уровнем уверенности.

Практический смысл такого сервиса: загрузить диагностический снимок fabric, сохранить его как версионированный лог, а затем быстро смотреть состав сети, количество портов, типы устройств, состояние портов и изменения между разными запусками диагностики.

## Стек

- Go 1.22+
- `net/http`
- PostgreSQL
- Docker Compose
- JSON-логи в stdout через `slog`

## Структура проекта

```text
cmd/app/              точка входа HTTP-сервиса
internal/config/      чтение настроек из env
internal/httpserver/  handlers и middleware
internal/models/      доменные структуры
internal/parser/      разбор zip-архива и секций db_csv
internal/storage/     PostgreSQL, миграции и запросы
migrations/           SQL-миграции
data/                 папка для архивов логов
```

## Настройки

| Переменная | Значение по умолчанию | Описание |
| --- | --- | --- |
| `PORT` | `8080` | Порт HTTP-сервера |
| `DATABASE_URL` | пусто | PostgreSQL DSN |
| `DATA_DIR` | `data` | Папка с архивами логов |
| `MIGRATIONS_DIR` | `migrations` | Папка с SQL-миграциями |
| `LOG_LEVEL` | `info` | Уровень логов: `debug`, `info`, `warn`, `error` |

В Docker Compose `DATA_DIR` указывает на `/app/data`, а локальная папка `./data` монтируется внутрь контейнера.

## Запуск через Docker Compose

Положите архив с логом в папку `data/`. Например:

```bash
cp /path/to/log.zip ./data/log.zip
```

Запустите приложение и базу:

```bash
docker compose up -d --build
```

Проверка здоровья:

```bash
curl http://localhost:8080/health
```

Просмотр логов приложения:

```bash
docker compose logs -f app
```

Остановка:

```bash
docker compose down
```

Остановка с удалением данных PostgreSQL:

```bash
docker compose down -v
```

Если порт PostgreSQL `5432` занят на машине, можно переопределить внешний порт:

```bash
POSTGRES_PORT=15432 docker compose up -d --build
```

Если Docker ругается на конфликт подсети, можно переопределить подсеть compose-сети:

```bash
DOCKER_SUBNET=172.30.30.0/24 docker compose up -d --build
```

## Локальный запуск без Docker

Поднимите PostgreSQL и передайте `DATABASE_URL`:

```bash
export DATABASE_URL='postgres://log_parser:log_parser@localhost:5432/log_parser?sslmode=disable'
export PORT=8080
export DATA_DIR=data
export MIGRATIONS_DIR=migrations
export LOG_LEVEL=debug

go run ./cmd/app
```

Миграции применяются автоматически при старте приложения.

## API

### Parse

Запрос:

```bash
curl -s -X POST http://localhost:8080/api/v1/parse/ \
  -H 'Content-Type: application/json' \
  -d '{"path":"log.zip"}'
```

Пример ответа:

```json
{
  "log_id": "0b6b8b1b-0f1e-4a30-b3bb-54f1a8d6c8e8",
  "status": "parsed",
  "nodes_count": 5,
  "ports_count": 151,
  "node_infos_count": 4
}
```

`path` должен быть путем относительно папки `data/`. Попытки выйти выше `data/`, например через `../`, отклоняются.

### Log Info

```bash
curl -s http://localhost:8080/api/v1/log/{log_id}
```

Возвращает статус лога, количество узлов/портов и даты загрузки/парсинга.

### Topology

```bash
curl -s http://localhost:8080/api/v1/topology/{log_id}
```

Возвращает список узлов, группы топологии и связи.

### Node

```bash
curl -s http://localhost:8080/api/v1/node/{node_id}
```

Возвращает детали узла и дополнительную информацию из `nodes_info`, если она есть.

### Ports

```bash
curl -s http://localhost:8080/api/v1/port/{node_id}
```

Возвращает список портов выбранного узла.

## Пример полного сценария

```bash
docker compose up -d --build

curl -s -X POST http://localhost:8080/api/v1/parse/ \
  -H 'Content-Type: application/json' \
  -d '{"path":"log.zip"}'

curl -s http://localhost:8080/api/v1/log/{log_id}
curl -s http://localhost:8080/api/v1/topology/{log_id}
curl -s http://localhost:8080/api/v1/node/{node_id}
curl -s http://localhost:8080/api/v1/port/{node_id}
```

Сначала нужно взять `log_id` из ответа parse-запроса. `node_id` можно взять из ответа topology-запроса.

## Проверки

```bash
go test ./...
go vet ./...
```

## База данных

Минимальные таблицы:

- `logs`
- `nodes`
- `ports`
- `nodes_info`

Все основные связи реализованы через foreign key:

- `nodes.log_id -> logs.id`
- `ports.log_id -> logs.id`
- `ports.node_id -> nodes.id`
- `nodes_info.log_id -> logs.id`
- `nodes_info.node_id -> nodes.id`

Миграции лежат в `migrations/` и применяются автоматически при старте приложения.
