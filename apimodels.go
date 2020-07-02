package main

type StateResponse struct {
	PeerId int `json:"peerId"`
	Status string `json:"status"`
}

type AccountResponse struct {
	AccountId      int    `json:"accountId"`
	ConnectCode    string    `json:"connectCode"`
	AuthKey        string    `json:"authKey"`
}

type ConnectArguments struct {
	ConnectCode    string    `json:"connectCode"`
}

type AcceptArguments struct {
	PeerId    int    `json:"peerId"`
}