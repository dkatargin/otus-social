package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

const (
	API_BASE_URL = "http://localhost:8080/api/v1"
	TOTAL_POSTS  = 100000
	MAX_USER_ID  = 10
)

// Шаблоны для генерации постов на русском языке
var postTemplates = []string{
	"Сегодня прекрасный день! %s. Настроение отличное! 😊",
	"Только что посетил %s. Впечатления незабываемые! Рекомендую всем.",
	"Читаю интересную книгу про %s. Кто-нибудь ещё читал что-то подобное?",
	"Завтракаю %s и думаю о планах на день. Кстати, кто что планирует на выходные?",
	"Посмотрел фильм про %s. Очень понравился! Кто ещё смотрел?",
	"Работаю над проектом связанным с %s. Очень увлекательно!",
	"Гуляю по городу и наслаждаюсь %s. Красота кругом!",
	"Учусь новому навыку - %s. Оказывается, это не так сложно, как казалось.",
	"Встретился с друзьями, обсуждали %s. Интересные точки зрения!",
	"Приготовил %s по новому рецепту. Получилось вкусно! Делюсь рецептом.",
	"Размышляю о %s. Жизнь полна удивительных моментов.",
	"Планирую поездку в %s. Кто-нибудь был там? Поделитесь впечлениями!",
	"Сегодня изучал %s. Столько всего интересного вокруг!",
	"Слушаю музыку и думаю о %s. Музыка вдохновляет на размышления.",
	"Тренировка прошла отлично! Особенно понравились упражнения с %s.",
	"Фотографирую %s. Получаются красивые кадры!",
	"Изучаю историю %s. Столько fascinating фактов!",
	"Готовлюсь к важной встрече по поводу %s. Волнуюсь, но уверен в успехе.",
	"Помогаю другу с %s. Приятно быть полезным!",
	"Открыл для себя новое увлечение - %s. Кто ещё этим занимается?",
}

var russianWords = []string{
	"программирование", "путешествия", "кулинария", "музыка", "фотография",
	"литература", "спорт", "природа", "искусство", "наука",
	"технологии", "образование", "здоровье", "семья", "дружба",
	"карьера", "творчество", "хобби", "медитация", "йога",
	"рисование", "танцы", "театр", "кино", "дизайн",
	"архитектура", "садоводство", "рыбалка", "велосипед", "горы",
	"море", "лес", "город", "деревня", "зима",
	"весна", "лето", "осень", "солнце", "дождь",
	"книги", "журналы", "новости", "история", "философия",
	"психология", "социология", "экономика", "политика", "экология",
}

var russianPlaces = []string{
	"Москва", "Санкт-Петербург", "Казань", "Екатеринбург", "Новосибирск",
	"Красноярск", "Сочи", "Калининград", "Владивосток", "Байкал",
	"Эрмитаж", "Третьяковская галерея", "Кремль", "Парк Горького", "ВДНХ",
	"Петергоф", "Царское Село", "Коломенское", "Кузьминки", "Сокольники",
	"центр города", "набережная", "парк", "музей", "театр",
	"библиотека", "кафе", "ресторан", "магазин", "рынок",
	"стадион", "бассейн", "спортзал", "кинотеатр", "галерея",
}

var russianFood = []string{
	"борщ", "пельмени", "блины", "каша", "суп",
	"салат", "мясо", "рыба", "овощи", "фрукты",
	"хлеб", "молоко", "сыр", "яйца", "картофель",
	"макароны", "рис", "гречка", "чай", "кофе",
	"компот", "сок", "вода", "торт", "пирог",
	"печенье", "конфеты", "мороженое", "йогурт", "творог",
}

type FriendRequest struct {
	FriendID int64 `json:"friend_id"`
}

type PostRequest struct {
	Content string `json:"content"`
}

func main() {
	log.Println("🚀 Начинаем генерацию тестовых данных...")

	// Инициализируем генератор случайных чисел
	rand.Seed(time.Now().UnixNano())
	gofakeit.Seed(time.Now().UnixNano())

	// 1. Устанавливаем дружбы между пользователями 1-5
	log.Println("📝 Создаём дружбы между пользователями 1-5...")
	createFriendships()

	// 2. Генерируем посты для пользователей 1-10
	log.Println("📱 Генерируем 100,000 постов для пользователей 1-10...")
	generatePosts()

	log.Println("✅ Генерация тестовых данных завершена!")
}

func createFriendships() {
	// Создаём все возможные дружбы между пользователями 1-5
	for userID := 1; userID <= 5; userID++ {
		for friendID := userID + 1; friendID <= 5; friendID++ {
			// Отправляем запрос на дружбу
			sendFriendRequest(int64(userID), int64(friendID))
			time.Sleep(10 * time.Millisecond) // Небольшая задержка

			// Одобряем дружбу
			approveFriendRequest(int64(friendID), int64(userID))
			time.Sleep(10 * time.Millisecond)

			log.Printf("✓ Дружба установлена: пользователь %d ↔ пользователь %d", userID, friendID)
		}
	}
}

func sendFriendRequest(userID, friendID int64) {
	url := fmt.Sprintf("%s/friends/add", API_BASE_URL)

	req := FriendRequest{FriendID: friendID}
	jsonData, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("❌ Ошибка создания запроса дружбы: %v", err)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-User-ID", fmt.Sprintf("%d", userID))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("❌ Ошибка отправки запроса дружбы: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		log.Printf("⚠️ Неожиданный статус при отправке запроса дружбы: %d", resp.StatusCode)
	}
}

func approveFriendRequest(userID, requestorID int64) {
	url := fmt.Sprintf("%s/friends/approve", API_BASE_URL)

	req := FriendRequest{FriendID: requestorID}
	jsonData, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("❌ Ошибка создания запроса одобрения: %v", err)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-User-ID", fmt.Sprintf("%d", userID))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("❌ Ошибка одобрения дружбы: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ Неожиданный статус при одобрении дружбы: %d", resp.StatusCode)
	}
}

func generatePosts() {
	successCount := 0
	errorCount := 0

	for i := 0; i < TOTAL_POSTS; i++ {
		// Выбираем случайного пользователя от 1 до 10
		userID := rand.Intn(MAX_USER_ID) + 1

		// Генерируем пост
		content := generatePostContent()

		if createPost(int64(userID), content) {
			successCount++
		} else {
			errorCount++
		}

		// Логируем прогресс каждые 1000 постов
		if (i+1)%1000 == 0 {
			log.Printf("📊 Прогресс: %d/%d постов создано (успешно: %d, ошибок: %d)",
				i+1, TOTAL_POSTS, successCount, errorCount)
		}

		// Небольшая задержка, чтобы не перегружать сервер
		if i%100 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	log.Printf("📈 Итого создано постов: %d успешно, %d ошибок", successCount, errorCount)
}

func generatePostContent() string {
	// Выбираем случайный шаблон
	template := postTemplates[rand.Intn(len(postTemplates))]

	// Выбираем случайное слово для вставки в шаблон
	var word string
	switch rand.Intn(3) {
	case 0:
		word = russianWords[rand.Intn(len(russianWords))]
	case 1:
		word = russianPlaces[rand.Intn(len(russianPlaces))]
	case 2:
		word = russianFood[rand.Intn(len(russianFood))]
	}

	// Формируем итоговый пост
	content := fmt.Sprintf(template, word)

	// Иногда добавляем дополнительные детали
	if rand.Float32() < 0.3 {
		additions := []string{
			" #жизнь #позитив",
			" 💭 Что думаете?",
			" Поделитесь своим мнением в комментариях!",
			" 🔥 Кто со мной согласен?",
			" ✨ Магия момента!",
			" 🎯 Цель на сегодня выполнена!",
			" 🌟 Каждый день - новая возможность!",
			" 📚 Учимся новому каждый день.",
			" 🤝 Вместе мы сильнее!",
			" 💪 Не сдаваемся!",
		}
		content += additions[rand.Intn(len(additions))]
	}

	return content
}

func createPost(userID int64, content string) bool {
	url := fmt.Sprintf("%s/posts/create", API_BASE_URL)

	req := PostRequest{Content: content}
	jsonData, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return false
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-User-ID", fmt.Sprintf("%d", userID))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusCreated
}
