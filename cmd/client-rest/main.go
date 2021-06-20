package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)


func main() {
	address := flag.String("server", "http://localhost:8080", "HTTP gateway url, http://localhost:8080")
	flag.Parse()

	t := time.Now().In(time.UTC)
	pfx := t.Format(time.RFC3339Nano)

	var body string

	resp, err := http.Post(*address + "/v1/auth", "application/json", strings.NewReader(fmt.Sprintf(`
		{
			"api": "v1",
			"auth": {
				"token": "some token text here (%s)",
				"description": "some token description here (%s)"
			}
		}
	`, pfx, pfx)))

	if err != nil {
		log.Fatalf("failed to call create method: %v", err)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	if err != nil {
		body = fmt.Sprintf("failed read create response body: %v", err)
	} else {
		body = string(bodyBytes)
	}

	log.Printf("Created response: Code=%d, Body=%s\n\n", resp.StatusCode, body)

	var created struct {
		API string `json:"api"`
		ID string `json:"id"`
	}

	err = json.Unmarshal(bodyBytes, &created)
	if err != nil { 
		log.Fatalf("failed to unmarshal JSON response of Created method: %v", err)
		fmt.Println("error: ", err)
	}
}