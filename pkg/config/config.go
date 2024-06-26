package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/rs/zerolog"

	"github.com/BurntSushi/toml"
	"github.com/creasty/defaults"
	"github.com/guregu/null/v5"
)

type Validator struct {
	Address          string `toml:"address"`
	ConsensusAddress string `toml:"consensus-address"`
}

func (v *Validator) Validate() error {
	if v.Address == "" {
		return errors.New("validator address is expected!")
	}

	return nil
}

type DenomInfo struct {
	Denom              string `toml:"denom"`
	DenomCoefficient   int64  `default:"1000000"            toml:"denom-coefficient"`
	DisplayDenom       string `toml:"display-denom"`
	CoingeckoCurrency  string `toml:"coingecko-currency"`
	DexScreenerChainID string `toml:"dex-screener-chain-id"`
	DexScreenerPair    string `toml:"dex-screener-pair"`
}

func (d *DenomInfo) Validate() error {
	if d.Denom == "" {
		return errors.New("empty denom name")
	}

	if d.Denom == "" {
		return errors.New("empty display denom name")
	}

	return nil
}

func (d *DenomInfo) DisplayWarnings(chain *Chain, logger *zerolog.Logger) {
	if d.CoingeckoCurrency == "" && (d.DexScreenerPair == "" || d.DexScreenerChainID == "") {
		logger.Warn().
			Str("chain", chain.Name).
			Str("denom", d.Denom).
			Msg("Currency code not set, not fetching exchange rate.")
	}
}

type DenomInfos []*DenomInfo

func (d DenomInfos) Find(denom string) *DenomInfo {
	for _, info := range d {
		if denom == info.Denom {
			return info
		}
	}

	return nil
}

type Chain struct {
	Name             string          `toml:"name"`
	LCDEndpoint      string          `toml:"lcd-endpoint"`
	BaseDenom        string          `toml:"base-denom"`
	Denoms           DenomInfos      `toml:"denoms"`
	BechWalletPrefix string          `toml:"bech-wallet-prefix"`
	Validators       []Validator     `toml:"validators"`
	Queries          map[string]bool `toml:"queries"`

	ProviderChainLCD string `toml:"provider-lcd-endpoint"`
}

func (c *Chain) IsConsumer() bool {
	return c.ProviderChainLCD != ""
}

func (c *Chain) Validate() error {
	if c.Name == "" {
		return errors.New("empty chain name")
	}

	if c.LCDEndpoint == "" {
		return errors.New("no LCD endpoint provided")
	}

	if len(c.Validators) == 0 {
		return errors.New("no validators provided")
	}

	for index, validator := range c.Validators {
		if err := validator.Validate(); err != nil {
			return fmt.Errorf("error in validator #%d: %s", index, err)
		}
	}

	for index, denomInfo := range c.Denoms {
		if err := denomInfo.Validate(); err != nil {
			return fmt.Errorf("error in denom #%d: %s", index, err)
		}
	}

	return nil
}

func (c *Chain) DisplayWarnings(logger *zerolog.Logger) {
	if c.BaseDenom == "" {
		logger.Warn().
			Str("chain", c.Name).
			Msg("Base denom is not set")
	}

	for _, denom := range c.Denoms {
		denom.DisplayWarnings(c, logger)
	}
}

func (c *Chain) QueryEnabled(query string) bool {
	if value, ok := c.Queries[query]; !ok {
		return true // all queries are enabled by default
	} else {
		return value
	}
}

type Config struct {
	LogConfig     LogConfig     `toml:"log"`
	TracingConfig TracingConfig `toml:"tracing"`
	ListenAddress string        `default:":9550" toml:"listen-address"`
	Timeout       int           `default:"10"    toml:"timeout"`
	Chains        []Chain       `toml:"chains"`
}

type LogConfig struct {
	LogLevel   string `default:"info"  toml:"level"`
	JSONOutput bool   `default:"false" toml:"json"`
}

type TracingConfig struct {
	Enabled                   null.Bool `default:"false"                     toml:"enabled"`
	OpenTelemetryHTTPHost     string    `toml:"open-telemetry-http-host"`
	OpenTelemetryHTTPInsecure null.Bool `default:"true"                      toml:"open-telemetry-http-insecure"`
	OpenTelemetryHTTPUser     string    `toml:"open-telemetry-http-user"`
	OpenTelemetryHTTPPassword string    `toml:"open-telemetry-http-password"`
}

func (c *TracingConfig) Validate() error {
	if c.Enabled.Bool && c.OpenTelemetryHTTPHost == "" {
		return errors.New("tracing is enabled, but open-telemetry-http-host is not provided")
	}

	return nil
}

func (c *Config) Validate() error {
	if err := c.TracingConfig.Validate(); err != nil {
		return fmt.Errorf("error in tracing config: %s", err)
	}

	if len(c.Chains) == 0 {
		return errors.New("no chains provided")
	}

	for index, chain := range c.Chains {
		if err := chain.Validate(); err != nil {
			return fmt.Errorf("error in chain %d: %s", index, err)
		}
	}

	return nil
}

func (c *Config) DisplayWarnings(logger *zerolog.Logger) {
	for _, chain := range c.Chains {
		chain.DisplayWarnings(logger)
	}
}

func (c *Config) GetCoingeckoCurrencies() []string {
	currencies := []string{}

	for _, chain := range c.Chains {
		for _, denom := range chain.Denoms {
			if denom.CoingeckoCurrency != "" {
				currencies = append(currencies, denom.CoingeckoCurrency)
			}
		}
	}

	return currencies
}

func GetConfig(path string) (*Config, error) {
	configBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	configString := string(configBytes)

	configStruct := Config{}
	if _, err = toml.Decode(configString, &configStruct); err != nil {
		return nil, err
	}

	if err = defaults.Set(&configStruct); err != nil {
		return nil, err
	}

	return &configStruct, nil
}
