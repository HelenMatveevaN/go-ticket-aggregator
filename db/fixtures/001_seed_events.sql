-- 1. Сначала удалим старую таблицу, если она создалась криво
DROP TABLE IF EXISTS events;

-- 2. Создаем правильную таблицу для мероприятий нашей Витрины
CREATE TABLE events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100) NOT NULL, -- Вот эта колонка, которую искал Go!
    age_restriction VARCHAR(10) NOT NULL,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    location VARCHAR(255) NOT NULL,
    min_price NUMERIC(10, 2) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'available'
);

-- 3. Наполняем Витрину тестовыми данными
INSERT INTO events (title, description, category, age_restriction, start_time, location, min_price, status)
VALUES 
('Рок-фестиваль "Максидром"', 'Главное рок-событие года. Живой звук, легендарные группы.', 'Концерты', '16+', '2026-08-15 18:00:00+03', 'Стадион Лужники, Москва', 3500.00, 'available'),
('Спектакль "Гамлет"', 'Современная постановка классической трагедии Шекспира.', 'Театр', '12+', '2026-08-20 19:00:00+03', 'МХТ им. Чехова, Москва', 1500.00, 'available'),
('Финальный матч Кубка', 'Борьба за главный трофей сезона. Главное спортивное противостояние.', 'Спорт', '0+', '2026-08-25 16:00:00+03', 'ВТБ Арена, Москва', 2000.00, 'available');
