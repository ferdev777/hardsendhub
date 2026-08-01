package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func main() {
	url := "https://api.resend.com/emails"
	req, _ := http.NewRequest("GET", url, nil)
	apiKey := os.Getenv("RESEND_API_KEY")
	req.Header.Add("Authorization", "Bearer "+apiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer res.Body.Close()
	
	body, _ := ioutil.ReadAll(res.Body)
	
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	
	fmt.Printf("Response keys: ")
	for k := range data {
		fmt.Printf("%s, ", k)
	}
	fmt.Println()
	
	if emails, ok := data["data"].([]interface{}); ok && len(emails) > 0 {
		fmt.Printf("Sample email: %+v\n", emails[0])
	} else {
		fmt.Println("No 'data' or empty array in response", string(body))
	}
}
