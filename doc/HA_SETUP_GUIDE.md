# Настройка HAProxy и Nginx для отказоустойчивости

## Обзор

Этот проект демонстрирует настройку высокодоступной (HA) архитектуры с использованием:
- **HAProxy** для балансировки PostgreSQL (master для записи, replicas для чтения)
- **Nginx** для балансировки HTTP/WebSocket трафика между инстансами приложений

## Быстрый старт

### 1. Настройка и запуск окружения

```bash
# Автоматическая настройка и запуск всех сервисов
./scripts/setup_ha_environment.sh
```

Скрипт выполнит:
- Копирование конфигурации для HAProxy
- Остановку старых контейнеров
- Сборку новых образов
- Запуск всех сервисов
- Проверку доступности

### 2. Проверка статуса

```bash
# Проверка статуса всех контейнеров
docker-compose ps

# Просмотр логов
docker-compose logs -f nginx
docker-compose logs -f haproxy
docker-compose logs -f backend-1
```

### 3. Доступные endpoints

- **API:** http://localhost:8080
- **HAProxy Stats:** http://localhost:8404/stats (username: admin, password: admin)
- **Nginx Status:** http://localhost:8082/nginx_status
- **RabbitMQ Management:** http://localhost:15672 (guest/guest)

## Тестирование отказоустойчивости

### Автоматический тест

```bash
# Запуск комплексного теста отказоустойчивости
./scripts/test_ha_availability.sh
```

Тест проверит:
1. Базовую доступность всех компонентов
2. Работу при отказе 1 backend инстанса
3. Работу при отказе 2 backend инстансов
4. Работу при отказе PostgreSQL реплики
5. Восстановление после отказов

### Ручное тестирование

```bash
# 1. Проверка нормальной работы
curl http://localhost:8080/health

# 2. Остановка одного backend
docker-compose stop backend-1

# 3. Проверка доступности (должна продолжать работать)
curl http://localhost:8080/health

# 4. Восстановление
docker-compose start backend-1
```

## Нагрузочное тестирование

### С использованием wrk

```bash
# Установка wrk (если не установлен)
brew install wrk  # macOS
# apt-get install wrk  # Ubuntu/Debian

# Запуск benchmark
./scripts/benchmark_ha.sh
```

### Ручной benchmark

```bash
# Базовый тест
wrk -t4 -c50 -d60s http://localhost:8080/health

# С отказом backend
docker-compose stop backend-1
wrk -t4 -c50 -d60s http://localhost:8080/health
docker-compose start backend-1
```

## Мониторинг

### HAProxy Statistics

Откройте http://localhost:8404/stats в браузере для просмотра:
- Статус PostgreSQL master и replicas
- Количество активных соединений
- Статистика запросов и ошибок
- Health check статус

### Nginx Status

```bash
curl http://localhost:8082/nginx_status
```

Показывает:
- Active connections
- Accepts/handled requests
- Reading/Writing/Waiting connections

### Docker Stats

```bash
# Мониторинг ресурсов контейнеров в реальном времени
docker stats

# Только backend сервисы
docker stats backend-1 backend-2 backend-3
```

## Архитектура

```
┌─────────┐
│ Client  │
└────┬────┘
     │
     ▼
┌──────────────┐
│    Nginx     │ (Load Balancer)
│   :8080      │
└──────┬───────┘
       │
       ├────────────────────────┐
       │                        │
       ▼                        ▼
┌──────────────┐        ┌──────────────┐
│  Backend 1-3 │        │  Dialogs 1-2 │
│   :8080      │        │   :8080      │
└──────┬───────┘        └──────┬───────┘
       │                       │
       └───────────┬───────────┘
                   │
                   ▼
           ┌──────────────┐
           │   HAProxy    │ (DB Load Balancer)
           │ :5432 :5433  │
           └──────┬───────┘
                  │
       ┏━━━━━━━━━━┻━━━━━━━━━━┓
       ▼                      ▼
┌──────────────┐      ┌──────────────┐
│ PG Master    │      │ PG Replicas  │
│   (Write)    │      │   (Read)     │
│   :5433      │      │ :5434, :5435 │
└──────────────┘      └──────────────┘
```

## Конфигурационные файлы

### HAProxy (`etc/haproxy.cfg`)

- **postgres_master** (порт 5432): направляет запросы на master для записи
- **postgres_replicas** (порт 5433): балансирует чтение между репликами
- **stats** (порт 8404): web-интерфейс статистики

### Nginx (`etc/nginx.conf`)

- **backend_cluster**: балансирует HTTP запросы между backend-1, backend-2, backend-3
- **dialogs_cluster**: балансирует между dialogs-1, dialogs-2
- Поддержка WebSocket для диалогов
- Retry logic при ошибках

### Application Config (`app.yaml.haproxy`)

```yaml
db:
  master:
    host: haproxy
    port: 5432  # Master для записи
  replicas:
    - host: haproxy
      port: 5433  # Replicas для чтения
```

## Сценарии отказов

### Отказ Backend инстанса

**Поведение:**
- Nginx автоматически исключает упавший инстанс
- Запросы перенаправляются на здоровые инстансы
- Производительность снижается на 33% (1 из 3)
- Доступность сохраняется 99%+

**Восстановление:**
- После запуска инстанс автоматически возвращается в пул
- Health checks (3 попытки каждые 2 секунды)

### Отказ PostgreSQL Replica

**Поведение:**
- HAProxy исключает недоступную реплику
- Чтение распределяется между оставшимися репликами
- Минимальное влияние на производительность
- Доступность сохраняется 99%+

**Восстановление:**
- Автоматическое возвращение после прохождения health check

### Отказ PostgreSQL Master

**Текущее поведение:**
- Запись недоступна
- Чтение продолжает работать через реплики

**Рекомендация:**
- Внедрить Patroni для автоматического failover master

## Результаты экспериментов

Полный отчет с результатами эксперимента смотрите в:
📄 **[doc/HA_PROXY_NGINX_EXPERIMENT.md](doc/HA_PROXY_NGINX_EXPERIMENT.md)**

### Краткие выводы

✅ **Доступность:** Улучшена с 99.9% до 99.95%+  
✅ **SPOF:** Устранены на уровне приложения  
✅ **Отказоустойчивость:** Система работает при отказе 33-66% backend инстансов  
✅ **Восстановление:** Автоматическое, без ручного вмешательства  

⚠️ **Стоимость:** 3x больше ресурсов для backend, дополнительная сложность  

## Troubleshooting

### Сервисы не запускаются

```bash
# Проверка логов
docker-compose logs

# Пересоздание контейнеров
docker-compose down -v
docker-compose up -d
```

### HAProxy не может подключиться к PostgreSQL

```bash
# Проверка health check
docker-compose exec haproxy nc -zv postgres-master 5432

# Проверка статуса PostgreSQL
docker-compose exec postgres-master pg_isready -U app_user
```

### Nginx возвращает 502 Bad Gateway

```bash
# Проверка статуса backend
docker-compose ps backend-1 backend-2 backend-3

# Перезапуск backend
docker-compose restart backend-1 backend-2 backend-3
```

### Высокая latency

```bash
# Проверка нагрузки на контейнеры
docker stats

# Увеличение ресурсов в docker-compose.yml
# Добавьте:
# resources:
#   limits:
#     cpus: '2'
#     memory: 2G
```

## Дополнительные команды

```bash
# Масштабирование backend
docker-compose up -d --scale backend=5

# Просмотр логов конкретного сервиса
docker-compose logs -f --tail=100 nginx

# Выполнение команды в контейнере
docker-compose exec backend-1 sh

# Остановка всех сервисов
docker-compose down

# Полная очистка (включая volumes)
docker-compose down -v
```

## Следующие шаги

1. **Monitoring & Alerting:**
   - Настроить Prometheus для сбора метрик
   - Добавить Grafana dashboards
   - Настроить алерты на критичные события

2. **HA для балансировщиков:**
   - Настроить keepalived для HAProxy
   - Настроить VRRP для Nginx
   - Virtual IP для failover

3. **PostgreSQL HA:**
   - Внедрить Patroni + etcd
   - Автоматический failover master
   - Настроить streaming replication

4. **Security:**
   - SSL/TLS termination на Nginx
   - Firewall rules
   - Rate limiting

5. **Performance:**
   - Connection pooling (PgBouncer)
   - Redis для кеширования
   - CDN для статики

## Полезные ссылки

- [HAProxy Documentation](http://www.haproxy.org/docs/)
- [Nginx Load Balancing](https://nginx.org/en/docs/http/load_balancing.html)
- [PostgreSQL High Availability](https://www.postgresql.org/docs/current/high-availability.html)
- [Docker Compose Documentation](https://docs.docker.com/compose/)

