package system_works

// TokenHoldersData - legacy format for token holders (used for converting old saved_holders.json format)
type TokenHoldersData struct {
	TokenIdentifier string            `json:"tokenIdentifier"`
	Ticker          string            `json:"ticker"`
	LastUpdated     string            `json:"lastUpdated"`
	Holders         map[string]Holder `json:"holders"`
	TotalCount      int               `json:"totalCount"`
}
