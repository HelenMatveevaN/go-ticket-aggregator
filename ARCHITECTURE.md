# Архитектура проекта Ticket Aggregator

В данном документе описано внутреннее устройство сервиса агрегации билетов, взаимодействие его компонентов и логика кэширования.

## 1. Диаграмма компонентов (C4 Component)

Сервис спроектирован по принципам чистой архитектуры (Clean Architecture). Слои строго изолированы друг от друга через интерфейсы.

```mermaid
flowchart TD
    Title[C4 Component Diagram — Агрегатор Билетов]
    Title --- handler

    %% Границы слоев (Subgraphs)
    subgraph api ["Интерфейс ввода (Веб-сервер)"]
        handler["HTTP Handler (net/http)"]
    end

    subgraph business ["Бизнес-логика"]
        uc["Event UseCase"]
    end

    subgraph storage ["Слой хранения данных"]
        pg_repo["Postgres Repository (pgxpool)"]
        redis_repo["Redis Cache Repository"]
    end

    subgraph infra ["Внешняя инфраструктура"]
        db_events[("PostgreSQL <br/> (Таблица: events)")]
        db_cache[("Redis <br/> (Кэш: showcase)")]
    end

    %% Связи и направления потоков данных
    handler -->|Вызов бизнес-логики| uc
    uc -->|1. Запрос кэша (Get)| redis_repo
    uc -->|2. При Cache Miss (Select)| pg_repo

    pg_repo --> db_events
    redis_repo --> db_cache

    %% Стилизация для красоты (Senior-touch)
    style Title fill:none,stroke:none,font-weight:bold,font-size:16px
    classDef database fill:#232f3e,stroke:#3f4f5f,color:#fff;
    class db_events,db_cache database;
```

## 2. Сценарий: Получение витрины мероприятий (Sequence Diagram)

Для защиты основной базы данных от высокой нагрузки на чтение реализован паттерн «Бронежилет» (Ленивое кэширование с ограниченным временем жизни TTL).

```mermaid
sequenceDiagram
    autonumber
    actor User as Клиент (curl / Браузер)
    participant H as HTTP Handler
    participant UC as Event UseCase
    participant R as Redis (Кэш)
    participant PG as Postgres (БД)

    User->>H: GET /events
    H->>UC: GetShowcaseEvents(ctx)
    
    UC->>R: Проверить наличие кэша витрины
    
    alt Cache Hit (Данные есть в Redis)
        R-->>UC: Возврат JSON-строки из памяти
        UC-->>H: Данные витрины
        H-->>User: HTTP 200 OK (Быстрый ответ из кэша)
    else Cache Miss (В Redis пусто)
        R-->>UC: Ошибка / nil (Пусто)
        Note over UC, PG: Слой UseCase защищает систему и идет в БД
        UC->>PG: Вызов SQL-запроса (SELECT * FROM events)
        PG-->>UC: Возврат строк из таблиц
        
        Note over UC, R: Сохраняем результат в кэш для следующих запросов
        UC->>R: Записать данные в кэш (TTL 5 минут)
        
        UC-->>H: Данные витрины
        H-->>User: HTTP 200 OK (Ответ из БД)
    end
```

## 3. Инфраструктурное развертывание (Docker Compose)
Вся экосистема автоматизирована и поднимается в изолированной сети `ticket-network`:
* **ticket-app**: Go-сервер (сокращен до 15.3 МБ через Multi-stage build).
* **postgres-db**: СУБД с выделенным томом `postgres_data` для персистентности данных и авто-накатом таблиц при старте.
* **redis-db**: In-memory хранилище кэша.
