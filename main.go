package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Message)

type Message struct {
	Username  string `json:"username"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Erro ao fazer upgrade da conexão:", err)
		return
	}
	defer ws.Close()

	clients[ws] = true

	// Envia todas as mensagens armazenadas no banco de dados para o novo cliente
	messages, err := loadMessages()
	if err != nil {
		fmt.Println("Erro ao carregar mensagens:", err)
		return
	}
	for _, msg := range messages {
		err := ws.WriteJSON(msg)
		if err != nil {
			fmt.Println("Erro ao enviar mensagem:", err)
			return
		}
	}

	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			fmt.Println("Erro ao ler mensagem:", err)
			delete(clients, ws)
			break
		}

		// Log para verificar a mensagem recebida
		fmt.Printf("Mensagem recebida: %#v\n", msg)

		// Adiciona o timestamp atual à mensagem
		msg.Timestamp = time.Now().Format("2006-01-02 15:04:05")

		// Salva a mensagem no banco de dados
		err = saveMessage(msg.Username, msg.Message, msg.Timestamp)
		if err != nil {
			fmt.Println("Erro ao salvar mensagem:", err)
			continue
		}

		broadcast <- msg
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				fmt.Println("Erro ao enviar mensagem:", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	var creds Credentials
	err := json.NewDecoder(r.Body).Decode(&creds)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	hashedPassword, err := hashPassword(creds.Password)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	err = createUser(creds.Username, hashedPassword)
	if err != nil {
		http.Error(w, "Username already taken", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var creds Credentials
	err := json.NewDecoder(r.Body).Decode(&creds)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if authenticateUser(creds.Username, creds.Password) {
		http.SetCookie(w, &http.Cookie{
			Name:  "username",
			Value: creds.Username,
			Path:  "/",
		})
		w.WriteHeader(http.StatusOK)
	} else {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
	}
}

func main() {
	defer db.Close() // Ensure the database connection is closed when the program exits

	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/register", handleRegister)
	http.HandleFunc("/login", handleLogin)

	go handleMessages()

	fmt.Println("Servidor iniciado na porta :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Erro ao iniciar o servidor:", err)
	}
}
