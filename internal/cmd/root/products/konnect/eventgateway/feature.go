package eventgateway

import (
	"os"
	"strings"
)

const enableEventGatewayEnv = "KONGCTL_ENABLE_EVENT_GATEWAY"

func eventGatewayViewEnabled() bool {
	value := strings.TrimSpace(os.Getenv(enableEventGatewayEnv))
	if value == "" {
		return false
	}
	value = strings.ToLower(value)
	return value == "1" || value == "true"
}
