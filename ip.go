package main

import (
	"fmt"
	"net/http"
	"encoding/json"
)

func bba() {
	// Faz uma requisição para a API ipify
	resp, err := http.Get("https://api.ipify.org?format=json")
	if err != nil {
		fmt.Println("Erro ao obter o IP público:", err)
		return
	}
	defer resp.Body.Close()

	// Decodifica a resposta JSON
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Println("Erro ao decodificar a resposta JSON:", err)
		return
	}

	// Exibe o IP público
	fmt.Println("IP público:", result["ip"])
}