# OTUS Social Network

Высоконагруженная социальная сеть с системой друзей и лентой постов, разработанная для курса OTUS Highload Architect.

## 🚀 Функциональность

### Основные возможности
- **Система пользователей** - регистрация, аутентификация, профили
- **Система друзей** - отправка заявок, подтверждение дружбы
- **Лента постов друзей** - кешированная лента с Redis и очередями
- **Масштабируемая архитектура** - репликация PostgreSQL, кеширование

### Технологический стек
- **Backend**: Go (Gin framework)
- **База данных**: PostgreSQL с репликацией (1 master + 2 slaves)
- **Кеширование**: Redis с sorted sets и очередями
- **Деплой**: Docker Compose
- **Тестирование**: Go test с SQLite in-memory

## 🏗️ Архитектура

### Компоненты системы
- **PostgreSQL Master** (порт 5433) - основная база данных для записи
- **PostgreSQL Slave 1** (порт 5434) - первая реплика для чтения  
- **PostgreSQL Slave 2** (порт 5435) - вторая реплика для чтения
- **Redis** (порт 6379) - кеширование лент и очереди обновлений
- **Backend API** (порт 8080) - Go приложение с автоматическим роутингом запросов

### Особенности ленты постов
- **Redis кеширование** - ленты хранятся в Sorted Sets
- **Асинхронные обновления** - через очереди Redis (5 воркеров)
- **Ограничение размера** - максимум 1000 постов в ленте
- **TTL кеша** - 24 часа с автоматической инвалидацией
- **Отказоустойчивость** - fallback на чтение из БД

## 🚀 Быстрый старт

### Запуск системы

```bash
# Запуск всех сервисов (PostgreSQL + Redis + Backend)
docker-compose up -d

# Проверка статуса всех контейнеров
docker-compose ps

# Просмотр логов
docker-compose logs -f backend
```

### Генерация тестовых данных

```bash
# Создание 100,000 постов и дружеских связей
./scripts/generate_test_data.sh
```

### Тестирование API

```bash
# Просмотр ленты пользователя
curl 'http://localhost:8080/api/v1/feed?limit=10' -H 'X-User-ID: 1'

# Создание поста
curl -X POST 'http://localhost:8080/api/v1/posts/create' \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: 1' \
  -d '{"content": "Мой новый пост!"}'

# Добавление друга
curl -X POST 'http://localhost:8080/api/v1/friends/add' \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: 1' \
  -d '{"friend_id": 2}'
```

## 📡 API Endpoints

### Аутентификация
- `POST /api/v1/auth/register` - регистрация пользователя
- `POST /api/v1/auth/login` - вход в систему
- `POST /api/v1/auth/logout` - выход из системы

### Пользователи
- `GET /api/v1/user/search` - поиск пользователей
- `GET /api/v1/user/get/:id` - получение профиля пользователя

### Друзья (требуют аутентификации)
- `POST /api/v1/friends/add` - отправить заявку в друзья
- `POST /api/v1/friends/approve` - подтвердить дружбу
- `DELETE /api/v1/friends/delete` - удалить из друзей
- `GET /api/v1/friends/list` - список друзей
- `GET /api/v1/friends/requests` - входящие заявки

### Посты и лента (требуют аутентификации)
- `POST /api/v1/posts/create` - создать пост
- `DELETE /api/v1/posts/:post_id` - удалить пост
- `GET /api/v1/feed` - получить ленту постов друзей

### Администрирование
- `DELETE /api/v1/admin/cache/feed/:user_id` - инвалидировать кеш ленты
- `POST /api/v1/admin/feed/rebuild/:user_id` - перестроить ленту из БД
- `POST /api/v1/admin/feed/rebuild-all` - перестроить все ленты
- `GET /api/v1/admin/queue/stats` - статистика очереди обновлений

## 🧪 Тестирование

### Запуск unit-тестов

```bash
# Все тесты
cd src && go test ./tests -v

# Тесты ленты постов
cd src && go test ./tests -run TestFeed -v

# Тесты с покрытием
cd src && go test ./tests -cover
```

### Нагрузочное тестирование

```bash
# Тестирование чтения
./scripts/load_test_read.sh

# Тестирование записи  
./scripts/load_test_write.sh

# Комплексное тестирование
./scripts/homework_load_test.sh
```

## 📚 Документация

### Детальная документация по компонентам:

- **[Система ленты постов](doc/FEED_SYSTEM.md)** - архитектура, кеширование, очереди
- **[Тесты ленты постов](doc/FEED_TESTS.md)** - описание тестов и их запуск
- **[Генератор тестовых данных](doc/DATA_GENERATOR.md)** - создание реалистичного контента
- **[Настройка репликации](doc/REPLICATION_SETUP.md)** - конфигурация PostgreSQL кластера
- **[Результаты нагрузочных тестов](doc/LOAD_TEST_RESULTS.md)** - производительность системы
- **[OpenAPI спецификация](doc/Backend-OpenAPI.json)** - полное описание REST API

## 🔧 Конфигурация

### Настройка подключений

Скопируйте и настройте конфигурационный файл:

```bash
# Для работы с Redis
cp etc/app.yaml.redis.example app.yaml

# Для базовой настройки
cp etc/app.yaml.example app.yaml
```

### Структура конфигурации

```yaml
db:
  master:
    host: "localhost"
    port: 5433
    user: "app_user"
    password: "app_password"
    dbname: "app_db"
  replicas:
    - host: "localhost"
      port: 5434
      # ...

redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0

backend:
  host: "0.0.0.0"  
  port: 8080
```

## 🎯 Домашние задания OTUS

### ДЗ "Репликация: практическое применение"

```bash
./scripts/09_homework_replication.sh full
```

Доступные команды:
- `start` - запустить систему
- `check-datasource` - проверить настройку реплицированного DataSource
- `setup-sync-replication` - настроить кворумную синхронную репликацию
- `demonstrate-routing` - демонстрация маршрутизации запросов на слейвы
- `test-read` - тестирование чтения
- `test-failover` - тестирование failover
- `cleanup` - очистка системы

### ДЗ "Кеширование"

```bash
# Генерация тестовых данных для демонстрации кеширования
./scripts/generate_test_data.sh

# Просмотр результатов работы кеша
curl http://localhost:8080/api/v1/admin/queue/stats
```

## 📊 Мониторинг и метрики

### Проверка состояния системы

```bash
# Статус репликации PostgreSQL
./scripts/monitor_replication.sh

# Статистика Redis
docker-compose exec redis redis-cli info stats

# Статистика очереди обновлений лент
curl http://localhost:8080/api/v1/admin/queue/stats

# Логи приложения
docker-compose logs -f backend
```

### Производительность

- **Пропускная способность**: ~500-600 постов/сек
- **Размер ленты**: до 1000 постов с TTL 24 часа
- **Воркеры очереди**: 5 асинхронных обработчиков
- **Кеш Redis**: Sorted Sets с автоматической инвалидацией

## 🏆 Особенности реализации

- **Отказоустойчивость** - работа без Redis (fallback на БД)
- **Масштабируемость** - горизонтальное масштабирование воркеров
- **Производительность** - кеширование лент и пагинация
- **Тестируемость** - комплексные unit и интеграционные тесты
- **Мониторинг** - детальная статистика и логирование

---

**Автор**: Проект разработан в рамках курса OTUS Highload Architect  
**Лицензия**: MIT
