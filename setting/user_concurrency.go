package setting

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
)

const UserConcurrencyOptionKey = "UserConcurrencyLimit"

// Stored as one option so a configuration update is applied atomically.
type UserConcurrencyConfig struct {
	Enabled      bool           `json:"enabled"`
	DefaultLimit int            `json:"default_limit"`
	Groups       map[string]int `json:"groups"`
	Users        map[string]int `json:"users"`
}

var userConcurrencyConfig atomic.Pointer[UserConcurrencyConfig]

func init() {
	limit := common.GetEnvOrDefault("USER_CONCURRENCY_LIMIT", 0)
	if limit < 0 || limit > 100000 {
		panic("USER_CONCURRENCY_LIMIT must be between 0 and 100000")
	}
	userConcurrencyConfig.Store(&UserConcurrencyConfig{
		Enabled: limit > 0, DefaultLimit: limit,
		Groups: map[string]int{}, Users: map[string]int{},
	})
}

func ParseUserConcurrencyConfig(value string) (*UserConcurrencyConfig, error) {
	var cfg *UserConcurrencyConfig
	if err := common.UnmarshalJsonStr(value, &cfg); err != nil {
		return nil, fmt.Errorf("invalid user concurrency configuration: %w", err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("user concurrency configuration must be an object")
	}
	if cfg.DefaultLimit < 0 || cfg.DefaultLimit > 100000 {
		return nil, fmt.Errorf("default concurrency limit must be between 0 and 100000")
	}
	for group, limit := range cfg.Groups {
		if strings.TrimSpace(group) == "" || group != strings.TrimSpace(group) || limit < 0 || limit > 100000 {
			return nil, fmt.Errorf("invalid concurrency limit for group %q", group)
		}
	}
	for user, limit := range cfg.Users {
		id, err := strconv.Atoi(user)
		if err != nil || id <= 0 || strconv.Itoa(id) != user || limit < 0 || limit > 100000 {
			return nil, fmt.Errorf("invalid concurrency limit for user %q", user)
		}
	}
	if cfg.Groups == nil {
		cfg.Groups = map[string]int{}
	}
	if cfg.Users == nil {
		cfg.Users = map[string]int{}
	}
	return cfg, nil
}

func UpdateUserConcurrencyConfig(value string) error {
	cfg, err := ParseUserConcurrencyConfig(value)
	if err != nil {
		return err
	}
	userConcurrencyConfig.Store(cfg)
	return nil
}

func UserConcurrencyConfigJSON() string {
	data, _ := common.Marshal(userConcurrencyConfig.Load())
	return string(data)
}

func GetUserConcurrencyLimit(userID int, userGroup string) int {
	cfg := userConcurrencyConfig.Load()
	if !cfg.Enabled {
		return 0
	}
	if limit, ok := cfg.Users[strconv.Itoa(userID)]; ok {
		return limit
	}
	if limit, ok := cfg.Groups[userGroup]; ok {
		return limit
	}
	return cfg.DefaultLimit
}
