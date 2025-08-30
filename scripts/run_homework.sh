# Нагрузочное тестирование чтения ДО настройки репликации
test_read_before() {
    step "Нагрузочное тестирование чтения (ДО оптимизации)"

    log "Запуск тестирования операций чтения..."
    cd "$PROJECT_DIR"

    # Временно изменяем параметры для быстрого тестирования
    export DURATION="10s"
    export THREADS=2
    export CONNECTIONS=10

    # Сохраняем результаты с префиксом "before"
    "$SCRIPTS_DIR/load_test_read.sh"

    if [ -f "user_get_results.txt" ]; then
        cp "user_get_results.txt" "user_get_results_before.txt"
        log "✓ user_get_results_before.txt создан"
    fi
    if [ -f "user_search_results.txt" ]; then
        cp "user_search_results.txt" "user_search_results_before.txt"
        log "✓ user_search_results_before.txt создан"
    fi
    if [ -f "mixed_results.txt" ]; then
        cp "mixed_results.txt" "mixed_results_before.txt"
        log "✓ mixed_results_before.txt создан"
    fi

    log "Результаты тестирования ДО оптимизации сохранены"
}

# Нагрузочное тестирование чтения ПОСЛЕ настройки репликации
test_read_after() {
    step "Нагрузочное тестирование чтения (ПОСЛЕ оптимизации)"

    log "Запуск тестирования операций чтения..."
    cd "$PROJECT_DIR"

    # Временно изменяем параметры для быстрого тестирования
    export DURATION="10s"
    export THREADS=2
    export CONNECTIONS=10

    "$SCRIPTS_DIR/load_test_read.sh"

    # Копируем результаты с суффиксом "after"
    if [ -f "user_get_results.txt" ]; then
        cp "user_get_results.txt" "user_get_results_after.txt"
        log "✓ user_get_results_after.txt создан"
    fi
    if [ -f "user_search_results.txt" ]; then
        cp "user_search_results.txt" "user_search_results_after.txt"
        log "✓ user_search_results_after.txt создан"
    fi
    if [ -f "mixed_results.txt" ]; then
        cp "mixed_results.txt" "mixed_results_after.txt"
        log "✓ mixed_results_after.txt создан"
    fi

    log "Результаты тестирования ПОСЛЕ оптимизации сохранены"
}

"demonstrate-routing")
            demonstrate_read_routing
            ;;
        "compare-results")
            compare_results
            ;;
        "start")
            echo "Использование: $0 {full|start|check-datasource|setup-sync-replication|demonstrate-routing|compare-results|test-read|test-failover|cleanup}"
            echo ""
            echo "Команды:"
            echo "  full                    - Выполнить полный сценарий домашнего задания"
            echo "  start                   - Запустить систему"
            echo "  check-datasource        - Проверить настройку реплицированного DataSource"
            echo "  setup-sync-replication  - Настроить кворумную синхронную репликацию"
            echo "  demonstrate-routing     - Демонстрация маршрутизации запросов на слейвы"
            echo "  compare-results         - Сравнить результаты ДО и ПОСЛЕ репликации"
            echo "  test-read              - Запустить тестирование чтения"
            echo "  test-failover          - Запустить тестирование failover"
            echo "  cleanup                - Очистить систему"
            echo ""
            echo "Примеры:"
            echo "  $0 full                # Полное выполнение задания"
            echo "  $0 start               # Только запуск системы"
            echo "  $0 check-datasource    # Проверка DataSource"
            echo "  $0 compare-results     # Сравнение производительности"
            exit 1
            ;;
