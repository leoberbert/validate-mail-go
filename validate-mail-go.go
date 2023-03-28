package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"gopkg.in/go-playground/validator.v9"
)

type EmailRequest struct {
	EmailAddress string `json:"emailAddress" validate:"required,email"`
}

type EmailResponse struct {
	EmailAddress string           `json:"emailAddress"`
	Status       int              `json:"status"`
	Bounce       *BounceComponent `json:"bounce,omitempty"`
}

type BounceComponent struct {
	Type   int    `json:"type"`
	Detail string `json:"detail"`
	Code   int    `json:"code"`
}

var validate *validator.Validate

func main() {
	validate = validator.New()

	// Carregando a lista negra
	blacklist, err := loadBlacklist()
	if err != nil {
		log.Fatal(err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		// Parseando o body da requisição
		var req EmailRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Validando o endereço de email
		err = validate.Struct(req)
		if err != nil {
			resp := &EmailResponse{
				EmailAddress: req.EmailAddress,
				Status:       2,
				Bounce: &BounceComponent{
					Type:   1,
					Detail: "Invalid mail format",
					Code:   990,
				},
			}
			writeJSONResponse(w, resp, http.StatusOK)
			return
		}

		// Checando se o endereço está na lista negra
		if isInBlacklist(blacklist, req.EmailAddress) {
			resp := &EmailResponse{
				EmailAddress: req.EmailAddress,
				Status:       2,
				Bounce: &BounceComponent{
					Type:   1,
					Detail: "Bad destination mailbox address",
					Code:   511,
				},
			}
			writeJSONResponse(w, resp, http.StatusOK)
			return
		}

		// Respondendo com sucesso
		resp := &EmailResponse{
			EmailAddress: req.EmailAddress,
			Status:       1,
		}
		writeJSONResponse(w, resp, http.StatusOK)
	}).Methods(http.MethodPost)

	// Configurando o servidor
	handler := cors.Default().Handler(r)
	addr := ":8080"
	fmt.Printf("Listening on %s...\n", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

// Carrega a lista negra de um arquivo
func loadBlacklist() ([]string, error) {
	filename := "blacklist.conf"
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("blacklist file %s not found", filename)
	}
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading blacklist file %s: %s", filename, err)
	}
	lines := strings.Split(string(data), "\n")
	// Removendo comentários e linhas vazias
	filteredLines := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		filteredLines = append(filteredLines, line)
	}
	return filteredLines, nil
}

var forbiddenWords []string

func init() {
	// Carregando a lista de palavras proibidas
	words, err := loadBlacklist()
	if err != nil {
		log.Fatal(err)
	}
	forbiddenWords = words
}

// Verifica se o endereço de email está na lista negra
func isInBlacklist(blacklist []string, email string) bool {
	for _, pattern := range blacklist {
		re := regexp.MustCompile(strings.ReplaceAll(pattern, "*", ".*"))
		if re.MatchString(email) {
			return true
		}
	}
	return false
}

// Escreve uma resposta em JSON para o cliente

func writeJSONResponse(w http.ResponseWriter, resp interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	enc := json.NewEncoder(w)
	enc.SetIndent("", " ")
	err := enc.Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
