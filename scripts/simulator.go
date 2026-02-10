// Payment Simulator — generates mock transactions for testing.
// Usage: go run scripts/simulator.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const baseURL = "http://localhost:8080/api/v1"

type Account struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	APIKey  string `json:"api_key"`
	Balance int64  `json:"balance"`
}

func main() {
	fmt.Println("=== Payment Simulator ===")
	fmt.Println("Creating test accounts...")

	accounts := createTestAccounts()
	if len(accounts) < 2 {
		fmt.Println("Failed to create accounts, exiting")
		return
	}

	fmt.Printf("Created %d accounts\n\n", len(accounts))

	// Generate random transactions.
	numTransactions := 20
	fmt.Printf("Generating %d transactions...\n\n", numTransactions)

	for i := 0; i < numTransactions; i++ {
		sender := accounts[rand.Intn(len(accounts))]
		receiver := accounts[rand.Intn(len(accounts))]
		for receiver.ID == sender.ID {
			receiver = accounts[rand.Intn(len(accounts))]
		}

		amount := int64(rand.Intn(10000) + 100)

		txn := initiatePayment(sender.ID, receiver.ID, amount)
		if txn == nil {
			continue
		}
		txnID := txn["id"].(string)
		fmt.Printf("[%d/%d] Initiated: %s (%s → %s) $%.2f\n",
			i+1, numTransactions, txnID[:8], sender.Name, receiver.Name, float64(amount)/100)

		time.Sleep(100 * time.Millisecond)

		// Authorize ~80% of transactions.
		if rand.Float64() < 0.8 {
			authorize(txnID)
			fmt.Printf("         Authorized: %s\n", txnID[:8])

			time.Sleep(100 * time.Millisecond)

			// Settle ~90% of authorized transactions.
			if rand.Float64() < 0.9 {
				settle(txnID)
				fmt.Printf("         Settled: %s\n", txnID[:8])
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("\n=== Simulation Complete ===")
}

func createTestAccounts() []Account {
	testAccounts := []struct {
		Name     string
		Email    string
		Type     string
		Balance  int64
	}{
		{"Alice Johnson", "alice@example.com", "user", 1000000},
		{"Bob Smith", "bob@example.com", "user", 500000},
		{"Charlie Corp", "charlie@corp.com", "merchant", 2000000},
		{"Diana's Store", "diana@store.com", "merchant", 750000},
		{"Eve Trading", "eve@trading.com", "merchant", 1500000},
	}

	var accounts []Account
	for _, ta := range testAccounts {
		body, _ := json.Marshal(map[string]interface{}{
			"name":     ta.Name,
			"email":    ta.Email,
			"type":     ta.Type,
			"currency": "USD",
			"balance":  ta.Balance,
		})

		resp, err := http.Post(baseURL+"/accounts", "application/json", bytes.NewReader(body))
		if err != nil {
			fmt.Printf("Failed to create account %s: %v\n", ta.Name, err)
			continue
		}

		var result Account
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		result.Name = ta.Name
		accounts = append(accounts, result)
	}

	return accounts
}

func initiatePayment(senderID, receiverID string, amount int64) map[string]interface{} {
	body, _ := json.Marshal(map[string]interface{}{
		"idempotency_key": uuid.New().String(),
		"sender_id":       senderID,
		"receiver_id":     receiverID,
		"amount":          amount,
		"currency":        "USD",
		"description":     "Simulated payment",
	})

	resp, err := http.Post(baseURL+"/payments", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

func authorize(txnID string) {
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/payments/%s/authorize", baseURL, txnID), nil)
	http.DefaultClient.Do(req)
}

func settle(txnID string) {
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/payments/%s/settle", baseURL, txnID), nil)
	http.DefaultClient.Do(req)
}
