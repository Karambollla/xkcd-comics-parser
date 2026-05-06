# XKCD Comics Parser

<div align="center">

![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![gRPC](https://img.shields.io/badge/gRPC-4285F4?style=for-the-badge&logo=google&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-336791?style=for-the-badge&logo=postgresql&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-2496ED?style=for-the-badge&logo=docker&logoColor=white)
![Hexagonal Architecture](https://img.shields.io/badge/Architecture-Hexagonal-orange?style=for-the-badge)

**Микросервисная система для поиска по комиксам xkcd.**  
Построена на принципах *Clean Architecture* (Ports & Adapters), что гарантирует независимость бизнес-логики от внешних инструментов.

</div>



<div align="center">
  <img src="guide.gif" alt="Guide" width="800px">
</div>



## Архитектура

Система разделена микросервисы, которые общаются между собой по **gRPC**:

*   **Words Service** — нормализация входящей фразы.
*   **Search Service** — поиск по бд/индексу.
*   **Update Service** — обновляет базу в соответствии с [xkcd.com](https://xkcd.com).
*   Также существует опция сборки метрик через VictoriaMetrics.


## Гайдлайн по Make

| Категория | Команда | Описание |
| :--- | :--- | :--- |
| **Инфраструктура** | `make up` | Поднять весь стек (Docker Compose) |
| | `make down` | Остановить все контейнеры |
| | `make clean` | Полная очистка: удаление контейнеров и томов БД |
| **Тесты** | `make test` | Полный CI-цикл: очистка → деплой → тесты → очистка |
| **Инструменты** | `make proto` | Перегенерация gRPC кода из `.proto` |
| | `make lint` | Проверка кода линтерами |
| | `make tools` | Установка dev-зависимостей (`protoc-gen-go` и др.) |
