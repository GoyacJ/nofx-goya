package api

import "strings"

// MaskSensitiveString Mask sensitive strings, showing only first 4 and last 4 characters
// Used to mask API Key, Secret Key, Private Key and other sensitive information
func MaskSensitiveString(s string) string {
	if s == "" {
		return ""
	}
	length := len(s)
	if length <= 8 {
		return "****" // String too short, hide everything
	}
	return s[:4] + "****" + s[length-4:]
}

// SanitizeModelConfigForLog Sanitize model configuration for log output
func SanitizeModelConfigForLog(models map[string]struct {
	Enabled         bool   `json:"enabled"`
	APIKey          string `json:"api_key"`
	CustomAPIURL    string `json:"custom_api_url"`
	CustomModelName string `json:"custom_model_name"`
}) map[string]interface{} {
	safe := make(map[string]interface{})
	for modelID, cfg := range models {
		safe[modelID] = map[string]interface{}{
			"enabled":           cfg.Enabled,
			"api_key":           MaskSensitiveString(cfg.APIKey),
			"custom_api_url":    cfg.CustomAPIURL,
			"custom_model_name": cfg.CustomModelName,
		}
	}
	return safe
}

// SanitizeExchangeConfigForLog Sanitize exchange configuration for log output
func SanitizeExchangeConfigForLog(exchanges map[string]struct {
	Enabled               bool   `json:"enabled"`
	APIKey                string `json:"api_key"`
	SecretKey             string `json:"secret_key"`
	Testnet               bool   `json:"testnet"`
	HyperliquidWalletAddr string `json:"hyperliquid_wallet_addr"`
	AsterUser             string `json:"aster_user"`
	AsterSigner           string `json:"aster_signer"`
	AsterPrivateKey       string `json:"aster_private_key"`
	LighterWalletAddr     string `json:"lighter_wallet_addr"`
	LighterPrivateKey     string `json:"lighter_private_key"`
	QMTGatewayURL         string `json:"qmt_gateway_url"`
	QMTAccountID          string `json:"qmt_account_id"`
	QMTGatewayToken       string `json:"qmt_gateway_token"`
	QMTMarket             string `json:"qmt_market"`
	AShareMarket          string `json:"ashare_market"`
	AShareTushareToken    string `json:"ashare_tushare_token"`
	AShareDataMode        string `json:"ashare_data_mode"`
	AShareWatchlist       string `json:"ashare_watchlist"`
}) map[string]interface{} {
	safe := make(map[string]interface{})
	for exchangeID, cfg := range exchanges {
		safeExchange := map[string]interface{}{
			"enabled": cfg.Enabled,
			"testnet": cfg.Testnet,
		}

		// Only add masked sensitive fields when they have values
		if cfg.APIKey != "" {
			safeExchange["api_key"] = MaskSensitiveString(cfg.APIKey)
		}
		if cfg.SecretKey != "" {
			safeExchange["secret_key"] = MaskSensitiveString(cfg.SecretKey)
		}
		if cfg.AsterPrivateKey != "" {
			safeExchange["aster_private_key"] = MaskSensitiveString(cfg.AsterPrivateKey)
		}
		if cfg.LighterPrivateKey != "" {
			safeExchange["lighter_private_key"] = MaskSensitiveString(cfg.LighterPrivateKey)
		}
		if cfg.QMTGatewayToken != "" {
			safeExchange["qmt_gateway_token"] = MaskSensitiveString(cfg.QMTGatewayToken)
		}
		if cfg.AShareTushareToken != "" {
			safeExchange["ashare_tushare_token"] = MaskSensitiveString(cfg.AShareTushareToken)
		}

		// Add non-sensitive fields directly
		if cfg.HyperliquidWalletAddr != "" {
			safeExchange["hyperliquid_wallet_addr"] = cfg.HyperliquidWalletAddr
		}
		if cfg.AsterUser != "" {
			safeExchange["aster_user"] = cfg.AsterUser
		}
		if cfg.AsterSigner != "" {
			safeExchange["aster_signer"] = cfg.AsterSigner
		}
		if cfg.LighterWalletAddr != "" {
			safeExchange["lighter_wallet_addr"] = cfg.LighterWalletAddr
		}
		if cfg.QMTGatewayURL != "" {
			safeExchange["qmt_gateway_url"] = cfg.QMTGatewayURL
		}
		if cfg.QMTAccountID != "" {
			safeExchange["qmt_account_id"] = cfg.QMTAccountID
		}
		if cfg.QMTMarket != "" {
			safeExchange["qmt_market"] = cfg.QMTMarket
		}
		if cfg.AShareMarket != "" {
			safeExchange["ashare_market"] = cfg.AShareMarket
		}
		if cfg.AShareDataMode != "" {
			safeExchange["ashare_data_mode"] = cfg.AShareDataMode
		}
		if cfg.AShareWatchlist != "" {
			safeExchange["ashare_watchlist"] = cfg.AShareWatchlist
		}

		safe[exchangeID] = safeExchange
	}
	return safe
}

// MaskEmail Mask email address, keeping first 2 characters and domain part
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "****" // Incorrect format
	}
	username := parts[0]
	domain := parts[1]
	if len(username) <= 2 {
		return "**@" + domain
	}
	return username[:2] + "****@" + domain
}
