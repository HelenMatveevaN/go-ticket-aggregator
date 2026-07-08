# === Этап 1: СБОРКА БИНАРНИКА (Heavy Builder) ===
#FROM golang:1.21-alpine AS builder
FROM dockerhub.timeweb.cloud/library/golang:1.21-alpine AS builder

#раб.папка
WORKDIR /app

# Копируем зависимости
COPY go.mod go.sum ./

#скачиваем библиотеки (в кэш)
RUN go mod download

#копируем весь исходный код проекта
COPY . .

#сборка 
#(CGO_ENABLED=0-отключ.зависимости от Си-библиотек, делает файл автономным)
#-ldflags="-s -w" отрезает отлад.инф-ю, уменьшая вес файла на треть
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o myservice ./cmd/app

# === Этап 2: ФИНАЛЬНЫЙ МИНИМАЛЬНЫЙ ОБРАЗ (Light Runner) ===
#новый чистый контейнер (весит 5 Мб)
#FROM alpine:3.19
FROM dockerhub.timeweb.cloud/library/alpine:3.19

# Использовать /app вместо /root/, чтобы у appuser был доступ
WORKDIR /app

#нюанс 1: добавляем корневые сертификаты (чтобы Go-бэк мог ходить по https во внешние api)
RUN apk --no-cache add ca-certificates

#нюанс 2: создаем безопасного пользователя, чтобы контейнер не запускался от root
RUN adduser -D -u 10001 appuser

#магия multistage: берем из 1го контейнера (builder) только 1 готовый файл myservice и копируем его сюда
#весь исходный код и тяжелый sdk остаются там и выбрасываются
COPY --from=builder /app/myservice .

# ДОБАВЛЯЕМ СТРОКУ: Копируем папку с конфигами из исходного кода
COPY config/ ./config/

# Явно даем права на запуск и меняем владельца всей папки /app
RUN chmod +x ./myservice && chown -R appuser:appuser /app

# Переключаемся на пользователя ПОСЛЕ всех настроек прав
USER appuser

#порт, через который слушаем go-приложение
EXPOSE 8080

#команда, кот.запустит сервис при старте контейнера
CMD ["./myservice"]