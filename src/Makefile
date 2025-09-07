# Makefile для удобного запуска тестов

.PHONY: test-setup test-down test test-verbose test-feed test-dialog test-clean

# Запуск тестового окружения
test-setup:
	docker-compose -f docker-compose.test.yaml up -d postgres-test redis-test
	@echo "Waiting for services to be ready..."
	@sleep 10

# Остановка тестового окружения
test-down:
	docker-compose -f docker-compose.test.yaml down -v

# Запуск всех тестов
test: test-setup
	@echo "Running tests..."
	go test ./tests -v || true
	@make test-down

# Запуск тестов с подробным выводом
test-verbose: test-setup
	@echo "Running tests with verbose output..."
	go test ./tests -v -count=1 || true
	@make test-down

# Запуск только тестов фида
test-feed: test-setup
	@echo "Running feed tests..."
	go test ./tests -run TestFeed -v || true
	@make test-down

# Запуск только тестов диалогов
test-dialog: test-setup
	@echo "Running dialog tests..."
	go test ./tests -run TestDialog -v || true
	@make test-down

# Очистка тестового окружения
test-clean:
	docker-compose -f docker-compose.test.yaml down -v --remove-orphans
	docker system prune -f

# Запуск интерактивного режима для отладки
test-debug: test-setup
	@echo "Test environment is ready. Run tests manually:"
	@echo "go test ./tests -run TestName -v"
	@echo "When done, run: make test-down"