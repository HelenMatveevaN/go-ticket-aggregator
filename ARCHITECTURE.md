# Архитектура проекта Ticket Aggregator (Букинг Мероприятий)

Сервис спроектирован по принципам чистой архитектуры (Clean Architecture) и поддерживает два параллельных сетевых интерфейса для разных бизнес-задач.


## Интерфейсы ввода и транспортные протоколы

1. **HTTP REST API (Порт :8085)** — используется для интеграции с веб-браузерами и внешними партнерами. Обслуживает Витрину (`/events`) и Кассу (`/bookings`).
2. **gRPC HTTP/2 Streaming (Порт :50051)** — используется для высоконагруженного мобильного приложения. Обслуживает реактивный стриминг свободных мест в зале в реальном времени.


## 1. Диаграмма компонентов (C4 Component)

Слои строго изолированы друг от друга через интерфейсы. 
Бизнес-логика ничего не знает о сетевых протоколах или конкретных базах данных.

```mermaid
flowchart TD
    %% Границы слоев (Subgraphs)
    subgraph api ["Интерфейс ввода (Сетевой транспорт)"]
        handler["HTTP Handler (net/http)<br/>Порт :8085"]
        grpc_server["gRPC Server (HTTP/2)<br/>Порт :50051"]
        interceptor["gRPC Interceptor<br/>(Трейсинг X-Trace-ID)"]
    end

    subgraph business ["Бизнес-логика"]
        uc["Event / Booking UseCase"]
    end

    subgraph storage ["Слой хранения данных"]
        pg_repo["Postgres Repository (pgxpool)"]
        redis_repo["Redis Cache Repository"]
        lru_cache["🧠 Кастомный Sharded LRU Cache<br/>(Для Highload-стриминга мест)"]
    end

    subgraph infra ["Внешняя инфраструктура"]
        db_events[("PostgreSQL <br/> Таблица: events / orders")]
        db_cache[("Redis <br/> Кэш: showcase")]
    end

    %% Связи и направления потоков данных без служебных символов
    handler -->|Вызов REST API| uc
    interceptor -->|Маркировка запроса| grpc_server
    grpc_server -->|Вызов gRPC Стриминга| uc

    uc -->|1. Запрос кэша витрины| redis_repo
    uc -->|2. При Cache Miss запрос к БД| pg_repo
    uc -->|3. Мгновенный запрос мест| lru_cache

    pg_repo --> db_events
    redis_repo --> db_cache

    %% Стилизация компонентов
    classDef database fill:#232f3e,stroke:#3f4f5f,color:#fff;
    class db_events,db_cache database;
```

## 2. Сценарий 1: Получение витрины мероприятий (HTTP REST)
Реализован паттерн «Бронежилет» (Ленивое кэширование с TTL) для защиты основной БД от нагрузок на чтение.

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

## 3. Сценарий 2: Стриминг свободных мест в зале (gRPC Stream)
Маркирует каждый входящий поток уникальной меткой `X-Trace-ID` через Интерцептор и пачками пушит сектора зала клиенту по HTTP/2, минуя накладные расходы текстового JSON.

```mermaid
sequenceDiagram
    autonumber
    actor App as 📱 Мобильное приложение
    participant I as gRPC Interceptor (server.go)
    participant S as 🚀 gRPC Server (server.go)
    participant C as 🧠 Кастомный LRU-Кэш (Неделя 3)

    App->>I: Вызов StreamAvailableTickets (EventID)
    Note over I: Проверяет или генерирует UUID.<br/>Кладет X-Trace-ID в контекст запроса.
    I->>S: Передача маркированного контекста

    loop Пачечный пуш данных по HTTP/2
        S->>C: Запрос мест для Сектора А
        C-->>S: Свободные места
        S->>App: stream.Send(ticket) -> Улетела Фан-зона
        Note over App: Пользователь мгновенно<br/>видит места на экране!
    end
```

## 4. Инфраструктурное развертывание (Docker Compose)
Вся экосистема автоматизирована и поднимается в изолированной сети `ticket-network`:
* **ticket-app**: Go-сервер (сокращен до 15.3 МБ через Multi-stage build).
* **postgres-db**: СУБД с выделенным томом `postgres_data` для персистентности данных и авто-накатом таблиц при старте.
* **redis-db**: In-memory хранилище кэша.


## 5. Взаимодействие компонентов в реальном времени (Что работает сейчас)
```mermaid
sequenceDiagram
    autonumber
    actor Fan as 📱 Мобильное приложение (КЛИЕНТ)
    actor Browser as 🌐 Браузер пользователя (КЛИЕНТ)
    participant HTTP as 🌍 HTTP-Сервер (:8085)
    participant gRPC as 🚀 gRPC-Сервер (:50051)
    participant Interceptor as gRPC Interceptor (server.go)
    participant UC as 🧠 Слой Бизнес-логики (UseCase)
    participant DB as 💾 Postgres / Redis

    %% Сценарий 1: Просмотр витрины по HTTP
    Browser->>HTTP: GET /events
    HTTP->>UC: Вызов eventUseCase.GetShowcaseEvents()
    UC->>DB: Запрос данных в Redis / Postgres
    DB-->>UC: Данные получены
    UC-->>HTTP: Возврат строки JSON
    HTTP-->>Browser: HTTP 200 OK (Пользователь видит список рок-фестивалей)

    %% Сценарий 2: Интерактивная карта зала по gRPC
    Fan->>gRPC: Вызов StreamAvailableTickets(eventId: 5)
    critical Сетевой перехват рантайма Go
        gRPC->>Interceptor: Перехват запроса до бизнес-логики
        Note over Interceptor: Проверяет x-trace-id.<br/>Если нет — генерирует UUID.<br/>Кладет метку в контекст.
        Interceptor->>gRPC: Передает управление обработчику сервера
        
        loop Пачечная отдача мест (Стриминг по HTTP/2)
            gRPC->>UC: Симуляция поиска свободных мест
            gRPC->>Fan: stream.Send(ticket) -> Улетела пачка Фан-зоны
            Note over Fan: На телефоне мгновенно<br/>загорается сектор!
        end
    end
    gRPC-->>Fan: Стрим завершен (Шлюз закрыт)
```

## 6. Технологический Roadmap (Что планируется сделать в будущем)
```mermaid
graph TD
    Client[📱Телефон фаната] -->|gRPC Стрим| Server[🚀 Наш gRPC Сервер]
    
    subgraph FUTURE_BLOCK [Целевая архитектура Highload-контура]
        Server -->|Неделя 3-4: Быстрый путь| Cache[(🧠 Кастомный Шардированный LRU-Кэш)]
        Server -->|Неделя 7: Медленный путь| Postgres[(💾 PostgreSQL с B-Tree и GIN индексами)]
        
        Server -->|Неделя 5-6: Обсервабилити| Prometheus[📊 Prometheus SDK]
        Prometheus -->|Экспорт метрик| Grafana[📈 Grafana Дашборды]
        
        Server -->|Неделя 6: Дебаг рантайма| Pprof[🔍 pprof на порту :6060]
        
        Server -->|Неделя 8: Касса| Outbox[(📦 Transactional Outbox Таблица)]
    end

    style FUTURE_BLOCK fill:#fff9c4,stroke:#fbc02d,stroke-width:2px
```

*   **Неделя 3–4 (Concurrency-кэш):** Замена симуляции стриминга на чтение из собственного потокобезопасного in-memory кэша на дженериках, разделенного на 256 шардов для минимизации конкуренции за мьютексы (Lock Contention).
*   **Неделя 5–6 (Обсервабилити продакшена):** Интеграция Prometheus SDK и вывод бизнес-метрик (RPS, Latency перцентили p99, Cache Hit Rate) на дашборды Grafana. Подключение встроенного профилировщика рантайма `pprof` на изолированном порту `:6060` для анализа Flame Graph памяти под нагрузкой.
*   **Неделя 7–8 (Оптимизация хранения и Надежность):** Проектирование физических схем таблиц базы данных, накат индексов `B-Tree` и `GIN` в фоне без блокировок продакшена (`CONCURRENTLY`), внедрение паттерна `Transactional Outbox` для атомарной отправки событий бронирования в брокеры сообщений.