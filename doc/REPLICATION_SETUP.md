# Домашнее задание: Настройка репликации PostgreSQL

## Описание

Этот проект реализует домашнее задание по курсу Highload Architect для настройки репликации PostgreSQL с мастером и двумя слейвами, включая нагрузочное тестирование и failover сценарии.

## Архитектура

- **PostgreSQL Master** (порт 5433) - основная база данных для записи
- **PostgreSQL Slave 1** (порт 5434) - первая реплика для чтения  
- **PostgreSQL Slave 2** (порт 5435) - вторая реплика для чтения
- **Backend API** (порт 8080) - Go приложение с роутингом запросов

## Настрой��а репликации

### 1. Запуск системы

```bash
# Запуск всех сервисов
docker-compose up -d

# Проверка статуса контейнеров
docker-compose ps
```

### 2. Проверка репликации

```bash
# Мониторинг репликации в реальном времени
./scripts/monitor_replication.sh

# Разовая проверка статуса
./scripts/manage_replication.sh status
```

## Нагрузочное тестирование

### Подготовка

Установите инструмент `wrk` для нагрузочного тестирования:

```bash
# macOS
brew install wrk

# Ubuntu/Debian
sudo apt-get install wrk
```

### Тестирование операций чтения

Запуск нагрузочного тестирования для endpoints `/user/get/{id}` и `/user/search`:

```bash
./scripts/load_test_read.sh
```

Скрипт выполняет:
- Тест endpoint `/user/get/{id}` с случайными ID
- Тест endpoint `/user/search` с различными параметрами поиска
- Смешанный тест обоих endpoints

Результаты сохраняются в файлы:
- `user_get_results.txt`
- `user_search_results.txt` 
- `mixed_results.txt`

### Тестирование операций записи

Запуск нагрузочного тестирования для создания пользователей:

```bash
./scripts/load_test_write.sh
```

Скрипт:
- Создает пользователей через endpoint `/user/register`
- Подсчитывает количество успешных записей
- Сохраняет результаты в `write_results.txt`

### Мониторинг во время тестирования

Во время выполнения нагрузочных тестов запустите мониторинг:

```bash
./scripts/monitor_replication.sh
```

## Настройка кворумной синхронной репликации

Проект уже настроен для синхронной репликации с кворумом. В конфигурации мастера установлено:

```
synchronous_standby_names = 'ANY 1 (postgres-slave-1,postgres-slave-2)'
synchronous_commit = on
```

Это означает, что транзакция будет подтверждена только после успешной записи на мастере и хотя бы одном из слейвов.

## Сценарий failover

### 1. Запуск нагрузки на запись

```bash
# В отдельном терминале запустите нагрузку на запись
./scripts/load_test_write.sh &
WRITE_PID=$!
```

### 2. Убийство одной из реплик

```bash
# Убиваем слейв 1
docker kill otus-social-postgres-slave-1-1

# Или убиваем слейв 2
docker kill otus-social-postgres-slave-2-1
```

### 3. Автоматический failover

```bash
# Выполнение полного сценария failover
./scripts/manage_replication.sh full-failover
```

Скрипт автоматически:
1. Определяет самый свежий слейв
2. Промоутит его до мастера
3. Переключает оставшийся слейв на новый мастер
4. Проверяет потери транзакций

### 4. Ручное управление failover

```bash
# Определить самый свежий слейв
./scripts/manage_replication.sh find-freshest

# Промоутить конкретный слейв
./scripts/manage_replication.sh promote postgres-slave-1

# Переключить слейв на новый мастер
./scripts/manage_replication.sh switch postgres-slave-2 postgres-slave-1

# Проверить потери транзакций
./scripts/manage_replication.sh check-loss
```

## API Endpoints

### Операции чтения (используют слейвы)

- `GET /api/v1/user/get/{id}` - получение пользователя по ID
- `GET /api/v1/user/search?first_name=X&last_name=Y` - поиск пользователей

### Операции записи (используют мастер)

- `POST /api/v1/user/register` - создание нового пользователя

Пример запроса:
```json
{
    "nickname": "testuser",
    "password": "password123",
    "first_name": "Иван",
    "last_name": "Иванов", 
    "birthday": "1990-01-01",
    "sex": "male",
    "city": "Москва"
}
```

## Мониторинг и метрики

### Статус репликации

```bash
# Проверка статуса репликации
./scripts/manage_replication.sh status

# Непрерывный мониторинг
./scripts/monitor_replication.sh
```

### Логи контейнеров

```bash
# Логи мастера
docker logs otus-social-postgres-master-1

# Логи слейвов
docker logs otus-social-postgres-slave-1-1
docker logs otus-social-postgres-slave-2-1

# Логи приложения
docker logs otus-social-backend-1
```

## Результаты тестирования

### Сравнение производительности

После выполнения тестов сравните результаты:

1. **До настройки репликации**: все запросы идут на мастер
2. **После настройки репликации**: чтение распределено между слейвами

Ожидаемые улучшения:
- Увеличение throughput для операций чтения
- Снижение нагрузки на мастер
- Улучшение времени отклика

### Анализ failover

Проверьте:
- Количество потерянных транзакций во время failover
- Время восстановления после промоутинга
- Консистентность данных между узлами

## Устранение неполадок

### Проблемы с репликацией

```bash
# Проверка статуса контейнеров
docker-compose ps

# Перезапуск слейва
docker-compose restart postgres-slave-1

# Пересоздание слейва
docker-compose down postgres-slave-1
docker volume rm otus-social_db-data-slave-1
docker-compose up -d postgres-slave-1
```

### Проблемы с подключением

```bash
# Проверка сетевых соединений
docker network ls
docker network inspect otus-social_otus

# Проверка портов
netstat -an | grep 543
```

## Полезные команды

```bash
# Подключение к базам данных
docker exec -it otus-social-postgres-master-1 psql -U app_user -d app_db
docker exec -it otus-social-postgres-slave-1-1 psql -U app_user -d app_db
docker exec -it otus-social-postgres-slave-2-1 psql -U app_user -d app_db

# Проверка конфигурации PostgreSQL
docker exec otus-social-postgres-master-1 cat /etc/postgresql/postgresql.conf

# Очистка данных
docker-compose down -v
docker system prune -f
```

## Конфигурация

### PostgreSQL Master
- Синхронная репликация с кворумом
- WAL level: replica
- Максимум отправителей WAL: 10
- Архивирование WAL логов

### PostgreSQL Slaves  
- Hot standby режим
- Feedback включен
- Автоматическое восстановление из WAL

### Приложение
- Автоматический роутинг запросов
- Read-only операции на слейвы
- Write операции на мастер
- Отслеживание транзакций записи
