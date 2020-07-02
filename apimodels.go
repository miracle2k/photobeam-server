package main

type AccountResponse struct {
	AccountId      int    `json:"accountId"`
	ConnectCode    string    `json:"connectCode"`
	AuthKey        string    `json:"authKey"`
}

type ConnectArguments struct {
	ConnectCode    string    `json:"connectCode"`
}