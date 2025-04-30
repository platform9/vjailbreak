package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	fs := http.FileServer(http.Dir("./swagger-ui"))
	http.Handle("/", fs)

	fmt.Println("✅ Swagger UI running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
