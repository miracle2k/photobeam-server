package main

/**
 * Represents the state of a connection. Returned by some API calls.
 *
 * Status: connected, pendingWithMe, pendingWithPeer
 */
type StateResponse struct {
	PeerId      int    `json:"peerId"`
	Status      string `json:"status"`
	ShouldFetch bool   `json:"shouldFetch"`
	ShouldPeerFetch bool   `json:"shouldPeerFetch"`
}

/**
 * Represents the state of the account/login. Returned by some API calls.
 */
type AccountResponse struct {
	AccountId   int    `json:"accountId"`
	ConnectCode string `json:"connectCode"`
	AuthKey     string `json:"authKey"`
}

// Arguments for various kinds of API calls.

type ConnectArguments struct {
	ConnectCode string `json:"connectCode"`
}

type AcceptArguments struct {
	PeerId int `json:"peerId"`

	// Set this to false to reject the connection request instead
	Accept bool `json:"accept"`
}
