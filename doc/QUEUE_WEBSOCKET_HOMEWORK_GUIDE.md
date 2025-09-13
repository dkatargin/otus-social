# Инструкция для проверки ДЗ "Очереди и отложенное выполнение #2"

## Краткое описание

Данное домашнее задание реализует систему создания постов с асинхронными WebSocket уведомлениями и отложенной материализацией ленты через очереди.

## Реализованные требования

### ✅ 1. Создание поста (метод /post/create)
- **Endpoint**: `POST /api/v1/posts/create`
- **Аутентификация**: через заголовок `X-User-ID` или `Authorization: Bearer test_token_N`
- **Тело запроса**: `{"content": "текст поста"}`

### ✅ 2. Асинхронное API с WebSocket
- **Endpoint**: `GET /api/v1/ws/feed` (WebSocket)
- **События**: при создании поста друзьями приходит событие `feed_posted`
- **Real-time обновления**: лента обновляется в реальном времени

### ✅ 3. Отложенная материализация ленты
- **Очередь**: Redis-based очередь `feed_update_queue`
- **Воркеры**: 5 воркеров обрабатывают задачи асинхронно
- **Обработка**: при создании поста задача добавляется в очередь

### ✅ 4. Целевая доставка (Routing Key из RabbitMQ)
- **Exchange**: тип "topic" с именем `feed_events`
- **Routing Key**: формат `user.{userID}` для адресной доставки
- **Consumer**: привязан к pattern `user.*`

## Архитектура системы

```
POST /api/v1/posts/create
         ↓
   PostService.CreatePost()
         ↓
   Сохранение в БД
         ↓
   Добавление в Redis Queue
         ↓
   Обработка воркерами
         ↓
   Публикация в RabbitMQ
         ↓
   WebSocket уведомления
```

## Быстрый запуск для проверки

### Шаг 1: Запуск тестового окружения

```bash
# Запуск всех сервисов
docker-compose -f docker-compose.test.yaml up -d

# Проверка статуса
docker-compose -f docker-compose.test.yaml ps
```

### Шаг 2: Создание схемы БД

```bash
# Создание таблиц в тестовой БД
docker exec -i otus-social-postgres-master-1 psql -U test_user -d social_test < etc/schema.sql
```

### Шаг 3: Запуск автоматического тестирования

```bash
# Интеграционные тесты
cd src && go test ./tests/integration_test.go -v

# WebSocket функциональность
chmod +x scripts/test_websocket.sh
./scripts/test_websocket.sh
```

## Ручное тестирование API

### 1. Проверка статистики очереди

```bash
curl http://localhost:8080/api/v1/admin/queue/stats
```

**Ожидаемый ответ:**
```json
{
  "queue_length": 0,
  "queue_name": "feed_update_queue",
  "worker_count": 5
}
```

### 2. Создание поста

```bash
curl -X POST http://localhost:8080/api/v1/posts/create \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -d '{"content": "Тестовый пост"}'
```

**Ожидаемый ответ:**
```json
{
  "id": 1,
  "user_id": 1,
  "content": "Тестовый пост",
  "created_at": "2025-09-13T21:00:00Z",
  "updated_at": "2025-09-13T21:00:00Z"
}
```

### 3. Получение ленты

```bash
curl -H "X-User-ID: 1" "http://localhost:8080/api/v1/feed?limit=10"
```

**Ожидаемый ответ:**
```json
{
  "posts": [
    {
      "id": 1,
      "user_id": 1,
      "user_name": "Test User",
      "content": "Тестовый пост",
      "created_at": "2025-09-13T21:00:00Z"
    }
  ],
  "has_more": false,
  "last_id": 1
}
```

## Тестирование WebSocket

### Установка websocat (если нужно)

```bash
# macOS
brew install websocat

# Или через cargo
cargo install websocat
```

### Подключение к WebSocket

```bash
websocat "ws://localhost:8080/api/v1/ws/feed" \
  --header "X-User-ID: 1"
```

**Ожидаемые сообщения:**
1. `{"event":"connected","message":"WebSocket connected"}`
2. При создании поста: `{"event":"feed_posted","user_id":1,"post_id":2,"author_id":1,"content":"новый пост","created_at":"..."}`

## Проверка логов системы

### Логи приложения

```bash
docker-compose -f docker-compose.test.yaml logs app-test --tail 50
```

**Что искать в логах:**
- `DEBUG: CreatePost called for userID=X`
- `Enqueued feed update task for user X`
- `Worker N processing task for user X`
- `RabbitMQ event published successfully`

### Логи RabbitMQ

```bash
docker-compose -f docker-compose.test.yaml logs rabbitmq-test --tail 20
```

## Структура проекта

### Ключевые файлы

- `src/services/posts.go` - основной сервис постов
- `src/services/queue.go` - система очередей
- `src/services/rabbitmq.go` - интеграция с RabbitMQ
- `src/services/wsmanager.go` - управление WebSocket
- `src/api/handlers/posts.go` - HTTP обработчики
- `src/api/handlers/ws.go` - WebSocket обработчик

### Конфигурация

- `docker-compose.test.yaml` - тестовое окружение
- `src/config/test.yaml` - тестовая конфигурация
- `etc/schema.sql` - схема базы данных

## Критерии оценки

### ✅ Обязательные требования
1. **Создание поста** - работает через API
2. **WebSocket уведомления** - события приходят в real-time
3. **Отложенная материализация** - очередь обрабатывает задачи
4. **Целевая доставка** - routing key обеспечивает адресность

### ✅ Дополнительные возможности
- Кеширование лент в Redis
- Fallback механизм для WebSocket
- Админские endpoints для управления
- Статистика очереди
- Интеграционные тесты

## Возможные проблемы и решения

### Проблема: WebSocket не подключается
**Решение:**
```bash
# Проверить статус сервиса
curl http://localhost:8080/api/v1/admin/queue/stats

# Проверить логи
docker-compose -f docker-compose.test.yaml logs app-test
```

### Проблема: Очередь не обрабатывается
**Решение:**
```bash
# Проверить Redis
docker-compose -f docker-compose.test.yaml logs redis-test

# Проверить воркеры в логах приложения
docker-compose -f docker-compose.test.yaml logs app-test | grep "Worker"
```

### Проблема: RabbitMQ недоступен
**Решение:**
```bash
# Проверить RabbitMQ
docker-compose -f docker-compose.test.yaml logs rabbitmq-test

# Система использует fallback через прямые WebSocket
```

## Команды для демонстрации

### 1. Полный цикл тестирования
```bash
# Запуск окружения
docker-compose -f docker-compose.test.yaml up -d

# Создание схемы
docker exec -i otus-social-postgres-master-1 psql -U test_user -d social_test < etc/schema.sql

# Автоматические тесты
cd src && go test ./tests/integration_test.go -v

# WebSocket тесты
./scripts/test_websocket.sh
```

### 2. Демонстрация реального времени
```bash
# Терминал 1: WebSocket подключение
websocat "ws://localhost:8080/api/v1/ws/feed" --header "X-User-ID: 1"

# Терминал 2: Создание поста
curl -X POST http://localhost:8080/api/v1/posts/create \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -d '{"content": "Live demo post"}'
```
