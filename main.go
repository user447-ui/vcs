package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"go.etcd.io/bbolt"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Разрешаем соединения с любых источников
	},
}

var clients = make(map[*websocket.Conn]bool)
var mu sync.Mutex
var db *bbolt.DB

// Открытие базы данных BoltDB
func openDB() {
	var err error
	db, err = bbolt.Open("data/messages.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
}

// Функция для сохранения сообщения в базе данных
func saveMessage(message string) {
	err := db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("messages"))
		if err != nil {
			return err
		}
		id, _ := bucket.NextSequence()
		key := []byte(fmt.Sprintf("%d", id))
		return bucket.Put(key, []byte(message))
	})
	if err != nil {
		log.Fatal(err)
	}
}

// Функция для чтения всех сообщений
func getMessages() []string {
	var messages []string
	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("messages"))
		if bucket == nil {
			return nil
		}
		bucket.ForEach(func(k, v []byte) error {
			messages = append(messages, string(v))
			return nil
		})
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return messages
}

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
	for _, msg := range getMessages() {
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

		fmt.Printf("Получено сообщение: %s\n", msg)

		// Сохраняем сообщение в базе данных
		mu.Lock()
		saveMessage(string(msg))
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
	// Открываем базу данных
	openDB()

	// Обрабатываем подключение по WebSocket
	http.HandleFunc("/ws", handleConnections)

	// Статический файл для фронта
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// Запуск сервера
	log.Println("Запуск сервера на http://localhost:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Ошибка при запуске сервера: ", err)
	}
}
