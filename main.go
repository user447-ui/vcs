package main

import (
	"fmt"
	"log"
	"net/http"
	"github.com/gorilla/websocket"
	"sync"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Разрешаем соединения с любых источников
	},
}

// Список подключенных клиентов
var clients = make(map[*websocket.Conn]bool)
var messages []string // Список сообщений
var mu sync.Mutex // Для синхронизации доступа к messages

func handleConnections(w http.ResponseWriter, r *http.Request) {
	// Устанавливаем WebSocket-соединение
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	// Добавляем клиента в список подключенных
	clients[conn] = true
	defer delete(clients, conn)

	// Отправляем всем новым клиентам старые сообщения
	mu.Lock()
	for _, msg := range messages {
		err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			log.Println("Ошибка отправки сообщения:", err)
			break
		}
	}
	mu.Unlock()

	// Обработка сообщений от клиента
	for {
		messageType, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			break
		}

		// Печатаем зашифрованное сообщение
		fmt.Printf("Получено сообщение: %s\n", msg)

		// Сохраняем сообщение
		mu.Lock()
		messages = append(messages, string(msg))
		mu.Unlock()

		// Отправляем сообщение всем подключенным клиентам
		mu.Lock()
		for client := range clients {
			err := client.WriteMessage(messageType, msg)
			if err != nil {
				log.Println(err)
				client.Close()
				delete(clients, client)
			}
		}
		mu.Unlock()
	}
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html") // Статический файл для фронта
	})

	// Обрабатываем подключение по WebSocket
	http.HandleFunc("/ws", handleConnections)

	// Запуск сервера
	log.Println("Запуск сервера на http://localhost:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Ошибка при запуске сервера: ", err)
	}
}