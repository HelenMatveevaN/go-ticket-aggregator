//gRPC Server Interceptor (вместо Middleware)

//от intercept — перехватывать

//код, который перехватывает сетевой запрос до того, 
//как он попадет в бизнес-логику, 
//делает с ним техническую работу (логирование, трейсинг) 
//и передает дальше.


package grpc

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "go-ticket-aggregator/api/v1"
)

// Ключ для хранения Trace ID в context.Context
type contextKey string
const TraceIDKey contextKey = "trace_id"

//генератор уникальных «меток» (Trace-ID)
// Сгенерировать простой легковесный UUID v4 без внешних зависимостей
// UUID - строка из 36 символов
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) //пакет "crypto/rand" обращается к ядру операционной системы
						//и заполняет эти 16 байт абсолютно случайными 
						//(криптографически стойкими) числами.
	
	// Конвертируем случайные байты в шестнадцатеричную строку с дефисами по стандарту UUID
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// StreamInterceptor для сквозного трейсинга стриминговых запросов
func StreamTraceInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		md, ok := metadata.FromIncomingContext(ctx) //вытаскиваем метаданные из сети
		
		var traceID string
		if ok {
			// Ищем клиентский Trace-ID в таблице метаданных
			if values := md.Get("x-trace-id"); len(values) > 0 {
				traceID = values[0]
			}
		}

		// Если в таблице метаданных ключа не было, генерируем свой UUID
		if traceID == "" {
			traceID = generateUUID()
		}

		// Кладем метку в рюкзак-контекст
		newCtx := context.WithValue(ctx, TraceIDKey, traceID)
		wrappedStream := &wrappedServerStream{ServerStream: ss, ctx: newCtx}

		log.Printf("[gRPC] 🟢 Начат стрим: %s | TraceID: %s", info.FullMethod, traceID)

		// Передаем управление дальше — к методу StreamAvailableTickets
		err := handler(srv, wrappedStream)

		log.Printf("[gRPC] 🔴 Завершен стрим: %s | TraceID: %s | Error: %v", info.FullMethod, traceID, err)
		return err
	}
}

// Вспомогательная структура для безопасной подмены контекста в потоке
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// -------------------------------------------------------------
// Реализация gRPC-сервера букинга мероприятий
// -------------------------------------------------------------

type EventServer struct {
	pb.UnimplementedEventTicketServiceServer
}

func NewEventServer() *EventServer {
	return &EventServer{}
}

// StreamAvailableTickets реализует бизнес-логику стриминга мест
// метод для внешнего клиента
func (s *EventServer) StreamAvailableTickets(req *pb.TicketRequest, stream pb.EventTicketService_StreamAvailableTicketsServer) error {
	// Извлекаем Trace ID из контекста для логирования
	traceID, _ := stream.Context().Value(TraceIDKey).(string)
	
	// Список секторов зала, которые мы отдаем порциями
	zones := []string{"Fan-Zone", "VIP-Sektor", "Parterre"}

	for _, zone := range zones {
		// Если клиент запросил конкретную зону, игнорируем остальные
		if req.ZoneType != "" && req.ZoneType != zone {
			continue
		}

		// Имитируем 100мс тяжелой работы (поиск свободных мест)
		time.Sleep(100 * time.Millisecond)

		// Генерируем тестовый ответ (в будущем тут будет ваш кастомный кэш)
		ticket := &pb.TicketResponse{
			TicketId:     fmt.Sprintf("tkt-%s-%d", zone, time.Now().UnixNano()),
			EventId:      req.EventId,
			Title:        "Солд-аут Рок Фестиваль",
			DateTime:     time.Now().Add(48 * time.Hour).Format(time.RFC3339),
			ZoneName:     zone,
			Row:          5,
			Seat:         17,
			PriceUnits:   650000, // Цена билета: 6500 рублей строго в копейках (int64)
			Currency:     "RUB",
			Status:       "AVAILABLE",
		}

		// Мгновенно выталкиваем билет в сеть клиенту
		if err := stream.Send(ticket); err != nil {
			return fmt.Errorf("ошибка отправки в стрим [TraceID: %s]: %w", traceID, err)
		}
	}

	return nil
}