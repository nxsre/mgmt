package vault

type Otpdata struct {
	IP       string `json:"ip"`
	Key      string `json:"key"`
	KeyType  string `json:"key_type"`
	Port     int    `json:"port"`
	Username string `json:"username"`
}

type Sshotpcred struct {
	LeaseId       string  `json:"lease_id"`
	LeaseDuration int     `json:"lease_duration"`
	Renewable     bool    `json:"renewable"`
	OTPData       Otpdata `json:"data"`
	Warnings      string  `json:"warnings"`
}
