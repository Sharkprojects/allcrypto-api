package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"github.com/rs/cors"
)

//go:embed index.html
var content embed.FS

type User struct {
	ID          int64   `json:"id"`
	Username    string  `json:"username"`
	Password    string  `json:"-"`
	IsBlocked   bool    `json:"is_blocked"`
	RenewalDate string  `json:"renewal_date"`
	IP          *string `json:"ip"`
	Indicacao   int     `json:"indicacao"`
}

type Response struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func conectarBD() (*sql.DB, error) {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("Variável de ambiente DATABASE_URL não definida.")
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar ao banco de dados: %v", err)
	}
	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("erro ao testar conexão com o banco: %v", err)
	}
	fmt.Println("Conexão com o banco de dados bem-sucedida!")
	return db, nil
}

func jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func listarUsuariosHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonResponse(w, http.StatusMethodNotAllowed, Response{Message: "Método não permitido"})
			return
		}
		rows, err := db.Query(`SELECT id, username, is_blocked, renewal_date, ip, indicacao FROM users ORDER BY id`)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, Response{Message: "Erro ao buscar usuários: " + err.Error()})
			return
		}
		defer rows.Close()
		var users []User
		for rows.Next() {
			var u User
			if err := rows.Scan(&u.ID, &u.Username, &u.IsBlocked, &u.RenewalDate, &u.IP, &u.Indicacao); err != nil {
				jsonResponse(w, http.StatusInternalServerError, Response{Message: "Erro ao ler dados do usuário: " + err.Error()})
				return
			}
			users = append(users, u)
		}
		jsonResponse(w, http.StatusOK, Response{Message: "Usuários listados com sucesso", Data: users})
	}
}

// =====================================================================================
//  INÍCIO DA ALTERAÇÃO: Função userActionsHandler com consultas case-insensitive
// =====================================================================================
func userActionsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonResponse(w, http.StatusMethodNotAllowed, Response{Message: "Método não permitido"})
			return
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			jsonResponse(w, http.StatusBadRequest, Response{Message: "Requisição inválida"})
			return
		}
		action, _ := payload["action"].(string)
		username, _ := payload["username"].(string)
		var query string
		var err error
		var result sql.Result

		// A cláusula WHERE foi modificada em todas as queries para usar LOWER()
		// Isso garante que a busca por 'username' não diferencie maiúsculas de minúsculas.
		switch action {
		case "inserirUsuario":
			password, _ := payload["password"].(string)
			renewalDate, _ := payload["renewal_date"].(string)
			// A inserção não precisa de alteração, pois cria um novo registro.
			query = `INSERT INTO users (username, password, is_blocked, renewal_date, indicacao) VALUES ($1, $2, false, $3, 0)`
			result, err = db.Exec(query, username, password, renewalDate)
		case "atualizarSenha":
			newPassword, _ := payload["new_password"].(string)
			query = `UPDATE users SET password = $1 WHERE LOWER(username) = LOWER($2)`
			result, err = db.Exec(query, newPassword, username)
		case "bloquearUsuario":
			isBlocked, _ := payload["is_blocked"].(bool)
			query = `UPDATE users SET is_blocked = $1 WHERE LOWER(username) = LOWER($2)`
			result, err = db.Exec(query, isBlocked, username)
		case "atualizarRenovacao":
			renewalDate, _ := payload["renewal_date"].(string)
			query = `UPDATE users SET renewal_date = $1 WHERE LOWER(username) = LOWER($2)`
			result, err = db.Exec(query, renewalDate, username)
		case "atualizarIndicacao":
			indicacao, ok := payload["indicacao"].(float64)
			if !ok {
				jsonResponse(w, http.StatusBadRequest, Response{Message: "Valor de indicação inválido"})
				return
			}
			query = `UPDATE users SET indicacao = $1 WHERE LOWER(username) = LOWER($2)`
			result, err = db.Exec(query, int(indicacao), username)
		case "atualizarIP":
			novoIP, _ := payload["novo_ip"].(string)
			query = `UPDATE users SET ip = $1 WHERE LOWER(username) = LOWER($2)`
			result, err = db.Exec(query, novoIP, username)
		default:
			jsonResponse(w, http.StatusBadRequest, Response{Message: "Ação desconhecida"})
			return
		}
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, Response{Message: "Erro ao executar ação '" + action + "': " + err.Error()})
			return
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			jsonResponse(w, http.StatusNotFound, Response{Message: "Nenhum usuário encontrado com o nome fornecido."})
			return
		}
		jsonResponse(w, http.StatusOK, Response{Message: "Ação '" + action + "' executada com sucesso!"})
	}
}
// =====================================================================================
//  FIM DA ALTERAÇÃO
// =====================================================================================


func main() {
	db, err := conectarBD()
	if err != nil {
		log.Fatalf("Erro fatal ao conectar ao banco de dados: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()

	mux.Handle("/", http.FileServer(http.FS(content)))
	mux.HandleFunc("/api/usuarios", listarUsuariosHandler(db))
	mux.HandleFunc("/api/user-action", userActionsHandler(db))

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type"},
	})

	handler := c.Handler(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Iniciando servidor na porta %s com CORS habilitado", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Erro ao iniciar o servidor: %v", err)
	}
}