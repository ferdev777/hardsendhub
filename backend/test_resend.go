package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func main() {
	url := "https://api.resend.com/emails"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Bearer re_KpAqWS89_No7zyh1vkDN4bMVW9qu4MZ6J")

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
