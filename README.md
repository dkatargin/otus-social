# otus-social

Проект по курсу OTUS Highload Architect

[OpenAPI 3.0](doc/Backend-OpenAPI.json)

## Зависимости
- [Go 1.21+](https://go.dev/dl/)
- [PostgreSQL 15+](https://www.postgresql.org/download/)
- [Python UV](https://docs.astral.sh/uv/) (для тестов и графиков)

## Тестирование нагрузки

Из каталога src запускаем GO-тесты для проверки скорости и сохранения результатов:

```bash
go test -bench=. -benchmem -run=^$ -count=10 ./tests/01_search_benchmark_test.go > search_bench.json
```

Строим графики:

```bash
uv run make_plot.py --input search_bench.json --output search_bench.png
```