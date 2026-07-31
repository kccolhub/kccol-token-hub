package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

func scanDatabaseJSON(value any, target any) error {
	switch value := value.(type) {
	case nil:
		return nil
	case []byte:
		if len(value) == 0 {
			return nil
		}
		return common.Unmarshal(value, target)
	case string:
		if value == "" {
			return nil
		}
		return common.UnmarshalJsonStr(value, target)
	default:
		return fmt.Errorf("unsupported database JSON value type %T", value)
	}
}
