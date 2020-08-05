package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func IntMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestIntMinBasic(t *testing.T) {
	ans := IntMin(2, -2)
	if ans != -2 {

		t.Errorf("IntMin(2, -2) = %d; want -2", ans)
	}
}

func TestIntMinTableDriven(t *testing.T) {
	var tests = []struct {
		a, b int
		want int
	}{
		{0, 1, 0},
		{1, 0, 0},
		{2, -2, -2},
		{0, -1, -1},
		{-1, 0, -1},
	}

	for _, tt := range tests {

		testname := fmt.Sprintf("%d,%d", tt.a, tt.b)
		t.Run(testname, func(t *testing.T) {
			ans := IntMin(tt.a, tt.b)
			if ans != tt.want {
				t.Errorf("got %d, want %d", ans, tt.want)
			}
		})
	}
}

func TestRegisterHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/register", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(RegisterHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `{"alive": true}`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}


func RunConnectHandler(from *Account, to *Account) (string, error) {
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(ConnectArguments{
		ConnectCode: to.ConnectCode,
	})

	req, err := http.NewRequest("GET", "/connect", buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", from.Key)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ConnectHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		panic(fmt.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK))
	}

	return rr.Body.String(), nil
}

func TestConnectHandler(t *testing.T) {
	db := Connect()
	defer db.Close()

	// Create a set of accounts
	account1 := &Account{
		Key: "key1",
		ConnectCode: "code1",
	}
	account2 := &Account{
		Key: "key2",
		ConnectCode: "code2",
	}
	account3 := &Account{
		Key: "key3",
		ConnectCode: "code3",
	}
	account4 := &Account{
		Key: "key4",
		ConnectCode: "code4",
	}
	err := db.Insert(account1, account2, account3, account4)
	if err != nil {
		panic(err)
	}

	// Prelink certain accounts
	LinkAccounts(db, account1, account2, "live")
	LinkAccounts(db, account3, account4, "live")

	// Connection request 1 to 3
	body, err := RunConnectHandler(account1, account3)
	expected := `{"alive": true}`
	if body != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", body, expected)
	}

	// Result: 1 loses its pair, 3 does not.

	// connection between 2 and 3 is removed
}